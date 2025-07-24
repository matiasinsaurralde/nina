package builder

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/matiasinsaurralde/nina/pkg/types"
)

type BuildpackGolang struct {
	*BaseBuildpack
	name string
}

var buildpackGolangDockerfile = `
# Build stage
FROM golang:1.24 AS builder
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
	return nil, nil
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
