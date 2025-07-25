package builder

import (
	"context"

	"github.com/docker/docker/client"
	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/types"
)

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

type BaseBuildpack struct {
	Config       *config.Config
	DockerClient *client.Client
}

func (b *BaseBuildpack) SetConfig(ctx context.Context, cfg *config.Config) error {
	b.Config = cfg
	return nil
}

func (b *BaseBuildpack) GetConfig() *config.Config {
	return b.Config
}

func (b *BaseBuildpack) SetDockerClient(cli *client.Client) {
	b.DockerClient = cli
}

func (b *BaseBuildpack) GetDockerClient() *client.Client {
	return b.DockerClient
}
