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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/matiasinsaurralde/nina/pkg/types"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
)

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

func (b *BuildpackGolang) Build(ctx context.Context, bundle *Bundle) (*types.DeploymentImage, error) {
	logger := bundle.GetLogger()
	request := bundle.GetRequest()
	tempDir := bundle.GetTempDir()

	// Find the directory containing main.go
	mainGoPath := ""
	err := filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "main.go" {
			mainGoPath = path
			return io.EOF // stop walking
		}
		return nil
	})
	if err != nil && err != io.EOF {
		logger.Error("Failed to search for main.go", "error", err)
		return nil, err
	}
	if mainGoPath == "" {
		return nil, errors.New("main.go not found in bundle")
	}
	mainDir := filepath.Dir(mainGoPath)

	// Write Dockerfile to the same directory as main.go
	dockerfilePath := filepath.Join(mainDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); err == nil {
		logger.Info("Overwriting existing Dockerfile", "path", dockerfilePath)
	}
	err = ioutil.WriteFile(dockerfilePath, []byte(buildpackGolangDockerfile), 0644)
	if err != nil {
		logger.Error("Failed to write Dockerfile", "error", err)
		return nil, err
	}
	logger.Info("Dockerfile written", "path", dockerfilePath)

	// Prepare Docker build context (the directory with main.go)
	contextDir := mainDir
	contextTar, err := archive.TarWithOptions(contextDir, &archive.TarOptions{})
	if err != nil {
		logger.Error("Failed to create build context tar", "error", err)
		return nil, err
	}
	defer contextTar.Close()

	// Build image name
	imageTag := fmt.Sprintf("nina-%s-%s", request.AppName, request.CommitHash)

	// Build the image
	dockerClient := b.GetDockerClient()
	buildOptions := dockertypes.ImageBuildOptions{
		Tags:       []string{imageTag},
		Dockerfile: "Dockerfile",
		Remove:     true,
		PullParent: true,
		// NoBuildArgs, NoCache, etc. for now
	}
	resp, err := dockerClient.ImageBuild(ctx, contextTar, buildOptions)
	if err != nil {
		logger.Error("Docker build failed", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Read and log the build output
	var buildOutput bytes.Buffer
	tee := io.TeeReader(resp.Body, &buildOutput)
	if err := jsonmessage.DisplayJSONMessagesStream(tee, os.Stdout, 0, false, nil); err != nil {
		logger.Error("Failed to display Docker build output", "error", err)
	}

	// Parse the last line for image ID
	var imageID string
	dec := json.NewDecoder(&buildOutput)
	for {
		var m map[string]interface{}
		if err := dec.Decode(&m); err != nil {
			break
		}
		if aux, ok := m["aux"].(map[string]interface{}); ok {
			if id, ok := aux["ID"].(string); ok {
				imageID = id
			}
		}
	}
	if imageID == "" {
		logger.Error("Failed to get image ID from build output")
		return nil, errors.New("failed to get image ID from build output")
	}

	// Inspect the image to get its size
	imageInspect, _, err := dockerClient.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		logger.Error("Failed to inspect built image", "error", err)
		return nil, err
	}

	deploymentImage := &types.DeploymentImage{
		ImageTag: imageTag,
		ImageID:  imageID,
		Size:     imageInspect.Size,
	}
	logger.Info("Docker image built successfully", "image_tag", imageTag, "image_id", imageID, "size", imageInspect.Size)
	return deploymentImage, nil
}

// Match checks if the buildpack matches the type of project:
func (b *BuildpackGolang) Match(ctx context.Context, bundle *Bundle) (bool, error) {
	tempDir := bundle.GetTempDir()
	logger := bundle.GetLogger()

	// Determine the base directory for Go files
	baseDir := tempDir

	// Check if go.mod is present in the root tempDir
	rootGoModPath := filepath.Join(tempDir, "go.mod")
	if _, err := os.Stat(rootGoModPath); os.IsNotExist(err) {
		logger.Debug("go.mod not found in root directory, searching for subdirectories", "temp_dir", tempDir)
		// go.mod not found in root, walk through tempDir to find the first directory
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			logger.Error("Failed to read temp directory", "temp_dir", tempDir, "error", err)
			return false, fmt.Errorf("failed to read temp directory: %s", tempDir)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				baseDir = filepath.Join(tempDir, entry.Name())
				logger.Debug("Found subdirectory, using as base directory", "subdirectory", entry.Name(), "base_dir", baseDir)
				break
			}
		}
	} else {
		logger.Debug("go.mod found in root directory, using root as base directory", "base_dir", baseDir)
	}

	// Check for go.mod in the determined base directory
	goModPath := filepath.Join(baseDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		logger.Debug("go.mod not found in base directory", "base_dir", baseDir)
		return false, errors.New("go.mod not found in base directory")
	}
	logger.Debug("go.mod found", "path", goModPath)

	// Check for go.sum in the determined base directory
	goSumPath := filepath.Join(baseDir, "go.sum")
	if _, err := os.Stat(goSumPath); os.IsNotExist(err) {
		logger.Debug("go.sum not found in base directory", "base_dir", baseDir)
		return false, errors.New("go.sum not found in base directory")
	}
	logger.Debug("go.sum found", "path", goSumPath)

	// Check for main.go in the determined base directory
	mainGoPath := filepath.Join(baseDir, "main.go")
	if _, err := os.Stat(mainGoPath); os.IsNotExist(err) {
		logger.Debug("main.go not found in base directory", "base_dir", baseDir)
		return false, errors.New("main.go not found in base directory")
	}
	logger.Debug("main.go found", "path", mainGoPath)

	// Use Go AST tools to read main.go and get the package name
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, mainGoPath, nil, parser.PackageClauseOnly)
	if err != nil {
		logger.Error("Failed to parse main.go", "path", mainGoPath, "error", err)
		return false, errors.New("failed to parse main.go")
	}

	// Check if the package name is "main"
	if node.Name.Name != "main" {
		logger.Debug("Package name is not 'main'", "package_name", node.Name.Name)
		return false, errors.New("package name is not 'main'")
	}
	logger.Debug("Package name is 'main', all checks passed")

	return true, nil
}

func (b *BuildpackGolang) Name() string {
	return b.name
}
