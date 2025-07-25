// Package builder provides functionality for building and packaging applications.
package builder

import (
	"context"

	"github.com/docker/docker/client"
	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/types"
)

// Buildpack defines the interface for buildpacks.
type Buildpack interface {
	// Build builds the project:
	Build(ctx context.Context, bundle *Bundle) (*types.DeploymentImage, error)
	// Match checks if the buildpack matches the type of project:
	Match(ctx context.Context, bundle *Bundle) (bool, error)
	// Name returns the name of the buildpack:
	Name() string
	SetConfig(ctx context.Context, cfg *config.Config) error
	GetConfig() *config.Config
	SetDockerClient(cli *client.Client)
	GetDockerClient() *client.Client
}

// BaseBuildpack provides common functionality for buildpacks.
type BaseBuildpack struct {
	Config       *config.Config
	DockerClient *client.Client
}

// SetConfig sets the configuration.
func (b *BaseBuildpack) SetConfig(_ context.Context, cfg *config.Config) error {
	b.Config = cfg
	return nil
}

// GetConfig returns the configuration.
func (b *BaseBuildpack) GetConfig() *config.Config {
	return b.Config
}

// SetDockerClient sets the Docker client.
func (b *BaseBuildpack) SetDockerClient(cli *client.Client) {
	b.DockerClient = cli
}

// GetDockerClient returns the Docker client.
func (b *BaseBuildpack) GetDockerClient() *client.Client {
	return b.DockerClient
}
