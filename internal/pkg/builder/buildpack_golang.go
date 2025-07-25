package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/types"
)

// BuildpackGolang represents a Golang buildpack.
type BuildpackGolang struct {
	*BaseBuildpack
	name string
}

var buildpackGolangDockerfile = `
# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o myapp

# Run stage
FROM scratch
ARG PORT=8080
EXPOSE ${PORT}
COPY --from=builder /app/myapp /myapp
ENTRYPOINT ["/myapp"]
`

// findMainGoFile finds the main.go file in the bundle
func (b *BuildpackGolang) findMainGoFile(tempDir string, log *logger.Logger) (string, error) {
	mainGoPath := ""
	err := filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk directory %s: %w", path, err)
		}
		if info.Name() == "main.go" {
			mainGoPath = path
			return io.EOF // stop walking
		}
		return nil
	})
	if err != nil && err != io.EOF {
		log.Error("Failed to search for main.go", "error", err)
		return "", fmt.Errorf("failed to walk directory: %w", err)
	}
	if mainGoPath == "" {
		return "", errors.New("main.go not found in bundle")
	}
	return mainGoPath, nil
}

// createDockerfile creates the Dockerfile in the main directory
func (b *BuildpackGolang) createDockerfile(mainDir string, log *logger.Logger) error {
	dockerfilePath := filepath.Join(mainDir, "Dockerfile")
	if _, statErr := os.Stat(dockerfilePath); statErr == nil {
		log.Info("Overwriting existing Dockerfile", "path", dockerfilePath)
	}
	writeErr := os.WriteFile(dockerfilePath, []byte(buildpackGolangDockerfile), 0o600)
	if writeErr != nil {
		log.Error("Failed to write Dockerfile", "error", writeErr)
		return fmt.Errorf("failed to write Dockerfile: %w", writeErr)
	}
	log.Info("Dockerfile written", "path", dockerfilePath)
	return nil
}

// buildDockerImage builds the Docker image
func (b *BuildpackGolang) buildDockerImage(ctx context.Context, contextDir, imageTag string, log *logger.Logger) (string, error) {
	contextTar, err := archive.TarWithOptions(contextDir, &archive.TarOptions{})
	if err != nil {
		log.Error("Failed to create build context tar", "error", err)
		return "", fmt.Errorf("failed to create tar archive: %w", err)
	}
	defer func() {
		if closeErr := contextTar.Close(); closeErr != nil {
			log.Error("Failed to close context tar", "error", closeErr)
		}
	}()

	dockerClient := b.GetDockerClient()
	buildOptions := dockertypes.ImageBuildOptions{
		Tags:       []string{imageTag},
		Dockerfile: "Dockerfile",
		Remove:     true,
		PullParent: true,
	}
	resp, err := dockerClient.ImageBuild(ctx, contextTar, buildOptions)
	if err != nil {
		log.Error("Docker build failed", "error", err)
		return "", fmt.Errorf("failed to build Docker image: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error("Failed to close response body", "error", closeErr)
		}
	}()

	// Read and log the build output
	var buildOutput bytes.Buffer
	tee := io.TeeReader(resp.Body, &buildOutput)
	if displayErr := jsonmessage.DisplayJSONMessagesStream(tee, os.Stdout, 0, false, nil); displayErr != nil {
		log.Error("Failed to display Docker build output", "error", displayErr)
	}

	// Parse the last line for image ID
	imageID := b.extractImageID(&buildOutput)
	if imageID == "" {
		log.Error("Failed to get image ID from build output")
		return "", errors.New("failed to get image ID from build output")
	}

	return imageID, nil
}

// extractImageID extracts the image ID from the build output
func (b *BuildpackGolang) extractImageID(buildOutput *bytes.Buffer) string {
	var imageID string
	dec := json.NewDecoder(buildOutput)
	for {
		var m map[string]interface{}
		if decodeErr := dec.Decode(&m); decodeErr != nil {
			break
		}
		if aux, ok := m["aux"].(map[string]interface{}); ok {
			if id, ok := aux["ID"].(string); ok {
				imageID = id
			}
		}
	}
	return imageID
}

// Build builds a deployment image from the bundle
func (b *BuildpackGolang) Build(ctx context.Context, bundle *Bundle) (*types.DeploymentImage, error) {
	log := bundle.GetLogger()
	request := bundle.GetRequest()
	tempDir := bundle.GetTempDir()

	// Find the directory containing main.go
	mainGoPath, err := b.findMainGoFile(tempDir, log)
	if err != nil {
		return nil, err
	}
	mainDir := filepath.Dir(mainGoPath)

	// Create Dockerfile
	if createErr := b.createDockerfile(mainDir, log); createErr != nil {
		return nil, createErr
	}

	// Build image name
	imageTag := fmt.Sprintf("nina-%s-%s", request.AppName, request.CommitHash)

	// Build the image
	imageID, buildErr := b.buildDockerImage(ctx, mainDir, imageTag, log)
	if buildErr != nil {
		return nil, buildErr
	}

	// Inspect the image to get its size
	dockerClient := b.GetDockerClient()
	imageInspect, err := dockerClient.ImageInspect(ctx, imageID)
	if err != nil {
		log.Error("Failed to inspect built image", "error", err)
		return nil, fmt.Errorf("failed to inspect Docker image: %w", err)
	}

	deploymentImage := &types.DeploymentImage{
		ImageTag: imageTag,
		ImageID:  imageID,
		Size:     imageInspect.Size,
	}
	log.Info("Docker image built successfully", "image_tag", imageTag, "image_id", imageID, "size", imageInspect.Size)
	return deploymentImage, nil
}

// Match checks if the buildpack matches the type of project:
func (b *BuildpackGolang) Match(_ context.Context, bundle *Bundle) (bool, error) {
	tempDir := bundle.GetTempDir()
	log := bundle.GetLogger()

	// Determine the base directory for Go files
	baseDir := tempDir

	// Check if go.mod is present in the root tempDir
	rootGoModPath := filepath.Join(tempDir, "go.mod")
	if _, statErr := os.Stat(rootGoModPath); os.IsNotExist(statErr) {
		log.Debug("go.mod not found in root directory, searching for subdirectories", "temp_dir", tempDir)
		// go.mod not found in root, walk through tempDir to find the first directory
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			log.Error("Failed to read temp directory", "temp_dir", tempDir, "error", err)
			return false, fmt.Errorf("failed to read temp directory: %s", tempDir)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				baseDir = filepath.Join(tempDir, entry.Name())
				log.Debug("Found subdirectory, using as base directory", "subdirectory", entry.Name(), "base_dir", baseDir)
				break
			}
		}
	} else {
		log.Debug("go.mod found in root directory, using root as base directory", "base_dir", baseDir)
	}

	// Check for go.mod in the determined base directory
	goModPath := filepath.Join(baseDir, "go.mod")
	if _, statErr := os.Stat(goModPath); os.IsNotExist(statErr) {
		log.Debug("go.mod not found in base directory", "base_dir", baseDir)
		return false, errors.New("go.mod not found in base directory")
	}
	log.Debug("go.mod found", "path", goModPath)

	// Check for go.sum in the determined base directory
	goSumPath := filepath.Join(baseDir, "go.sum")
	if _, statErr := os.Stat(goSumPath); os.IsNotExist(statErr) {
		log.Debug("go.sum not found in base directory", "base_dir", baseDir)
		return false, errors.New("go.sum not found in base directory")
	}
	log.Debug("go.sum found", "path", goSumPath)

	// Check for main.go in the determined base directory
	mainGoPath := filepath.Join(baseDir, "main.go")
	if _, statErr := os.Stat(mainGoPath); os.IsNotExist(statErr) {
		log.Debug("main.go not found in base directory", "base_dir", baseDir)
		return false, errors.New("main.go not found in base directory")
	}
	log.Debug("main.go found", "path", mainGoPath)

	// Use Go AST tools to read main.go and get the package name
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, mainGoPath, nil, parser.PackageClauseOnly)
	if err != nil {
		log.Error("Failed to parse main.go", "path", mainGoPath, "error", err)
		return false, errors.New("failed to parse main.go")
	}

	// Check if the package name is "main"
	if node.Name.Name != "main" {
		log.Debug("Package name is not 'main'", "package_name", node.Name.Name)
		return false, errors.New("package name is not 'main'")
	}
	log.Debug("Package name is 'main', all checks passed")

	return true, nil
}

// Name returns the name of the buildpack.
func (b *BuildpackGolang) Name() string {
	return b.name
}
