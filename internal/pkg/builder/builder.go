package builder

import (
	"context"
	"errors"

	"github.com/docker/docker/client"
	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/types"
)

var availableBuildpacks = []Buildpack{
	&BuildpackGolang{BaseBuildpack: &BaseBuildpack{}, name: "golang"},
}

// Builder is the interface that wraps the MatchBuildpack method.
type Builder interface {
	ExtractBundle(ctx context.Context, req *types.BuildRequest) (*Bundle, error)
	MatchBuildpack(ctx context.Context, req *types.BuildRequest) (Buildpack, error)
	Build(ctx context.Context, bundle *Bundle, buildpack Buildpack) (*types.DeploymentImage, error)
	Init(ctx context.Context, cfg *config.Config, log *logger.Logger) error
	SetDockerClient(cli *client.Client)
	GetDockerClient() *client.Client
}

// BaseBuilder is the base implementation of the Builder interface.
type BaseBuilder struct {
	cfg          *config.Config
	logger       *logger.Logger
	buildpacks   map[string]Buildpack
	dockerClient *client.Client // Docker Engine API client (private)
}

func (b *BaseBuilder) Init(ctx context.Context, cfg *config.Config, log *logger.Logger) error {
	b.cfg = cfg
	b.logger = log
	b.buildpacks = make(map[string]Buildpack)
	for _, buildpack := range availableBuildpacks {
		buildpack.SetConfig(ctx, cfg)
		buildpack.SetDockerClient(b.dockerClient)
		b.buildpacks[buildpack.Name()] = buildpack
	}
	b.logger.Info("Builder initialized", "buildpacks_count", len(availableBuildpacks))
	return nil
}

func (b *BaseBuilder) ExtractBundle(ctx context.Context, req *types.BuildRequest) (*Bundle, error) {
	b.logger.Info("Extracting bundle", "app_name", req.AppName, "commit_hash", req.CommitHash)
	bundle, err := NewBundle(req, b.logger)
	if err != nil {
		b.logger.Error("Failed to extract bundle", "app_name", req.AppName, "error", err)
		return nil, err
	}
	b.logger.Info("Bundle extracted successfully", "app_name", req.AppName, "temp_dir", bundle.tempDir)
	return bundle, nil
}

// MatchBuildpack matches the buildpack for the given request.
func (b *BaseBuilder) MatchBuildpack(ctx context.Context, req *types.BuildRequest) (Buildpack, error) {
	var err error
	var bundle *Bundle
	bundle, err = b.ExtractBundle(ctx, req)
	if err != nil {
		return nil, err
	}
	for name, buildpack := range availableBuildpacks {
		isMatched, err := buildpack.Match(ctx, bundle)
		if err != nil {
			b.logger.Error("Failed to match buildpack", "buildpack_name", name, "error", err)
			continue
		}
		if isMatched {
			b.logger.Info("Buildpack matched", "buildpack_name", name)
			return buildpack, nil
		}
	}
	return nil, errors.New("no buildpack matched")
}

func (b *BaseBuilder) Build(ctx context.Context, bundle *Bundle, buildpack Buildpack) (*types.DeploymentImage, error) {
	deploymentImage, err := buildpack.Build(ctx, bundle)
	if err != nil {
		b.logger.Error("Failed to build", "error", err)
		return nil, err
	}
	return deploymentImage, nil
}

func (b *BaseBuilder) SetDockerClient(cli *client.Client) {
	b.dockerClient = cli
}

func (b *BaseBuilder) GetDockerClient() *client.Client {
	return b.dockerClient
}
