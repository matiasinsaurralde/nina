// Package engine provides HTTP Engine server functionality for the Nina application.
package engine

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
	"github.com/matiasinsaurralde/nina/internal/pkg/builder"
	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/store"
	"github.com/matiasinsaurralde/nina/pkg/types"
)

// Engine defines the interface for the Engine server
type Engine interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	SetConfig(cfg *config.Config)
	GetConfig() *config.Config
	SetDockerClient(cli *client.Client)
	GetDockerClient() *client.Client
}

// BaseEngine implements the Engine interface
type BaseEngine struct {
	config       *config.Config
	logger       *logger.Logger
	store        *store.Store
	builder      builder.Builder
	router       *gin.Engine
	server       *http.Server
	dockerClient *client.Client
}

// NewEngine creates a new Engine server instance
func NewEngine(cfg *config.Config, log *logger.Logger, st *store.Store) Engine {
	// Set Gin mode based on log level
	if log.GetLevel() == logger.LevelDebug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
	router.Use(loggerMiddleware(log))

	// Initialize Docker client with default options
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Error("Failed to initialize Docker client", "error", err)
		return nil
	}
	log.Info("Docker client initialized successfully")

	// Initialize builder
	b := &builder.BaseBuilder{}
	b.SetDockerClient(dockerClient)
	if err := b.Init(context.Background(), cfg, log); err != nil {
		log.Error("Failed to initialize builder", "error", err)
		// Continue without builder for now
	}

	server := &BaseEngine{
		config:       cfg,
		logger:       log,
		store:        st,
		builder:      b,
		router:       router,
		dockerClient: dockerClient,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// Start starts the Engine server
func (s *BaseEngine) Start(ctx context.Context) error {
	s.server = &http.Server{
		Addr:              s.config.GetServerAddr(),
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       5 * time.Minute,
	}

	s.logger.Info("Starting Engine server", "addr", s.config.GetServerAddr())

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Failed to start server", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	return s.Stop(context.Background())
}

// Stop stops the Engine server
func (s *BaseEngine) Stop(ctx context.Context) error {
	if s.server != nil {
		s.logger.Info("Stopping Engine server")
		return fmt.Errorf("failed to shutdown server: %w", s.server.Shutdown(ctx))
	}
	return nil
}

// SetConfig sets the configuration
func (s *BaseEngine) SetConfig(cfg *config.Config) {
	s.config = cfg
}

// GetConfig returns the current configuration
func (s *BaseEngine) GetConfig() *config.Config {
	return s.config
}

// setupRoutes sets up the API routes
func (s *BaseEngine) setupRoutes() {
	// Health check
	s.router.GET("/health", s.healthHandler)

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	v1.POST("/provision", s.provisionHandler)
	v1.POST("/deploy", s.deployHandler)
	v1.POST("/build", s.buildHandler)
	v1.GET("/builds", s.listBuildsHandler)
	v1.DELETE("/builds/:id", s.deleteBuildsHandler)
	v1.GET("/deployments", s.listDeploymentsHandler)
	v1.GET("/deployments/:id", s.getDeploymentHandler)
	v1.DELETE("/deployments/:id", s.deleteDeploymentHandler)
	v1.GET("/deployments/:id/status", s.getDeploymentStatusHandler)
}

// healthHandler handles health check requests
func (s *BaseEngine) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"service":   "nina-engine",
	})
}

// provisionHandler handles container provisioning requests
func (s *BaseEngine) provisionHandler(c *gin.Context) {
	var req store.ProvisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Validate request
	if req.Name == "" || req.Image == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Name and image are required",
		})
		return
	}

	// Create deployment
	deployment, err := s.store.CreateDeployment(c.Request.Context(), &req)
	if err != nil {
		s.logger.Error("Failed to create deployment", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create deployment",
		})
		return
	}

	// Update status to running (simulating container start)
	go func() {
		time.Sleep(2 * time.Second) // Simulate container startup time
		if err := s.store.UpdateDeploymentStatus(context.Background(), deployment.ID, "running"); err != nil {
			s.logger.Error("Failed to update deployment status", "id", deployment.ID, "error", err)
		}
	}()

	c.JSON(http.StatusCreated, deployment)
}

// deployHandler handles deployment requests
func (s *BaseEngine) deployHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	var req types.DeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid deployment request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Validate request
	if req.AppName == "" || req.CommitHash == "" {
		s.logger.Error("Missing required fields in deployment request", "app_name", req.AppName, "commit_hash", req.CommitHash)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "App name and commit hash are required",
		})
		return
	}

	s.logger.Info("Processing deployment request", "app_name", req.AppName, "commit_hash", req.CommitHash)

	// Check if build exists and is built
	build, err := s.store.GetBuild(ctx, req.CommitHash)
	if err != nil {
		s.logger.Error("Build not found", "commit_hash", req.CommitHash, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Build not found for the given commit hash",
		})
		return
	}

	if build.Status != types.BuildStatusBuilt {
		s.logger.Error("Build not ready", "commit_hash", req.CommitHash, "status", build.Status)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Build is not ready for deployment",
		})
		return
	}

	// Create deployment record
	deployment, err := s.store.CreateNewDeployment(ctx, &req)
	if err != nil {
		s.logger.Error("Failed to create deployment record", "app_name", req.AppName, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create deployment record",
		})
		return
	}

	// Update deployment status to deploying
	if err := s.store.UpdateNewDeploymentStatus(ctx, req.AppName, types.DeploymentStatusDeploying); err != nil {
		s.logger.Error("Failed to update deployment status to deploying", "error", err)
	}

	// Deploy containers in background
	go func() {
		if err := s.deployContainers(context.Background(), req.AppName, build.ImageTag); err != nil {
			s.logger.Error("Failed to deploy containers", "app_name", req.AppName, "error", err)
			if updateErr := s.store.UpdateNewDeploymentStatus(context.Background(), req.AppName, types.DeploymentStatusFailed); updateErr != nil {
				s.logger.Error("Failed to update deployment status to failed", "error", updateErr)
			}
		}
	}()

	c.JSON(http.StatusCreated, deployment)
}

// deployContainers deploys containers for the given app
func (s *BaseEngine) deployContainers(ctx context.Context, appName, imageTag string) error {
	s.logger.Info("Starting container deployment", "app_name", appName, "image_tag", imageTag)

	// Find available port (simple implementation - in production you'd want a more sophisticated port management)
	port := 5001 // For now, use a fixed port

	// Create container configuration
	containerConfig := &container.Config{
		Image: imageTag,
		Env: []string{
			fmt.Sprintf("PORT=%d", port),
		},
		ExposedPorts: nat.PortSet{
			nat.Port(fmt.Sprintf("%d/tcp", port)): struct{}{},
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			nat.Port(fmt.Sprintf("%d/tcp", port)): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: fmt.Sprintf("%d", port),
				},
			},
		},
	}

	// Create container
	resp, err := s.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, fmt.Sprintf("nina-%s", appName))
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	containerID := resp.ID
	s.logger.Info("Container created", "container_id", containerID, "app_name", appName)

	// Start container
	if err := s.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	s.logger.Info("Container started", "container_id", containerID, "app_name", appName)

	// Create container info
	container := types.Container{
		ContainerID: containerID,
		ImageTag:    imageTag,
		Address:     "localhost",
		Port:        port,
	}

	// Update deployment with container information and set status to ready
	if err := s.store.UpdateNewDeploymentWithContainers(ctx, appName, []types.Container{container}, types.DeploymentStatusReady); err != nil {
		return fmt.Errorf("failed to update deployment with containers: %w", err)
	}

	s.logger.Info("Deployment completed successfully", "app_name", appName, "container_id", containerID)
	return nil
}

// deleteDeploymentHandler handles deployment deletion requests
func (s *BaseEngine) deleteDeploymentHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Deployment ID is required",
		})
		return
	}

	// Try to get deployment using the new types structure first
	deployment, err := s.store.GetNewDeployment(c.Request.Context(), id)
	if err != nil {
		// If not found, try the old structure
		_, oldErr := s.store.GetDeployment(c.Request.Context(), id)
		if oldErr != nil {
			s.logger.Error("Failed to get deployment", "id", id, "error", err)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Deployment not found",
			})
			return
		}
		// For old deployments, just delete from store (no containers to clean up)
		if err := s.store.DeleteDeployment(c.Request.Context(), id); err != nil {
			s.logger.Error("Failed to delete deployment", "id", id, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to delete deployment",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Deployment deleted successfully",
			"id":      id,
		})
		return
	}

	// Clean up containers for new deployment type
	for _, cont := range deployment.Containers {
		if cont.ContainerID != "" {
			s.logger.Info("Removing container", "container_id", cont.ContainerID, "app_name", deployment.AppName)
			if err := s.dockerClient.ContainerRemove(c.Request.Context(), cont.ContainerID, container.RemoveOptions{Force: true}); err != nil {
				s.logger.Error("Failed to remove container", "container_id", cont.ContainerID, "error", err)
				// Continue with other containers even if one fails
			}
		}
	}

	// Delete deployment from store
	if err := s.store.DeleteNewDeployment(c.Request.Context(), id); err != nil {
		s.logger.Error("Failed to delete deployment", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete deployment",
		})
		return
	}

	s.logger.Info("Deployment deleted successfully", "id", id, "app_name", deployment.AppName, "containers_removed", len(deployment.Containers))
	c.JSON(http.StatusOK, gin.H{
		"message":            "Deployment deleted successfully",
		"id":                 id,
		"containers_removed": len(deployment.Containers),
	})
}

// getDeploymentHandler handles deployment retrieval requests
func (s *BaseEngine) getDeploymentHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Deployment ID is required",
		})
		return
	}

	deployment, err := s.store.GetDeployment(c.Request.Context(), id)
	if err != nil {
		s.logger.Error("Failed to get deployment", "id", id, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Deployment not found",
		})
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// getDeploymentStatusHandler handles deployment status requests
func (s *BaseEngine) getDeploymentStatusHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Deployment ID is required",
		})
		return
	}

	deployment, err := s.store.GetDeployment(c.Request.Context(), id)
	if err != nil {
		s.logger.Error("Failed to get deployment", "id", id, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Deployment not found",
		})
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// listDeploymentsHandler handles deployment listing requests
func (s *BaseEngine) listDeploymentsHandler(c *gin.Context) {
	appName := c.Query("app_name")

	var deployments []*types.Deployment
	var err error

	if appName != "" {
		// Get deployments by app name
		deployments, err = s.store.ListNewDeploymentsByAppName(c.Request.Context(), appName)
	} else {
		// Get all deployments
		deployments, err = s.store.ListNewDeployments(c.Request.Context())
	}

	if err != nil {
		s.logger.Error("Failed to list deployments", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list deployments",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deployments": deployments,
		"count":       len(deployments),
	})
}

// buildHandler handles build requests
func (s *BaseEngine) buildHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	var req types.BuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid build request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Validate request
	if req.AppName == "" || req.BundleContents == "" {
		s.logger.Error("Missing required fields in build request", "app_name", req.AppName)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "App name and bundle contents are required",
		})
		return
	}

	s.logger.Info("Processing build request", "app_name", req.AppName, "commit_hash", req.CommitHash)

	// Create build record in Redis
	_, buildErr := s.store.CreateBuild(ctx, &req)
	if buildErr != nil {
		s.logger.Error("Failed to create build record", "app_name", req.AppName, "error", buildErr)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create build record",
		})
		return
	}

	// Extract bundle
	bundle, err := s.builder.ExtractBundle(ctx, &req)
	if err != nil {
		s.logger.Error("Failed to extract bundle", "app_name", req.AppName, "error", err)
		// Update build status to failed
		if updateErr := s.store.UpdateBuildStatus(ctx, req.CommitHash, types.BuildStatusFailed); updateErr != nil {
			s.logger.Error("Failed to update build status to failed", "error", updateErr)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to extract bundle",
		})
		return
	}

	// Match buildpack
	buildpack, err := s.builder.MatchBuildpack(ctx, &req)
	if err != nil {
		s.logger.Error("Failed to match buildpack", "app_name", req.AppName, "error", err)
		// Update build status to failed
		if updateErr := s.store.UpdateBuildStatus(ctx, req.CommitHash, types.BuildStatusFailed); updateErr != nil {
			s.logger.Error("Failed to update build status to failed", "error", updateErr)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to match buildpack",
		})
		return
	}

	if buildpack == nil {
		s.logger.Warn("No matching buildpack found", "app_name", req.AppName)
		// Update build status to failed
		if updateErr := s.store.UpdateBuildStatus(ctx, req.CommitHash, types.BuildStatusFailed); updateErr != nil {
			s.logger.Error("Failed to update build status to failed", "error", updateErr)
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No matching buildpack found for this project type",
		})
		return
	}

	s.logger.Info("Buildpack matched", "app_name", req.AppName, "buildpack", buildpack.Name())

	// Update build status to building
	if err := s.store.UpdateBuildStatus(ctx, req.CommitHash, types.BuildStatusBuilding); err != nil {
		s.logger.Error("Failed to update build status to building", "error", err)
	}

	// Build the project
	deployment, err := buildpack.Build(ctx, bundle)
	if err != nil {
		s.logger.Error("Failed to build project", "app_name", req.AppName, "error", err)
		// Update build status to failed
		if updateErr := s.store.UpdateBuildStatus(ctx, req.CommitHash, types.BuildStatusFailed); updateErr != nil {
			s.logger.Error("Failed to update build status to failed", "error", updateErr)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to build project",
		})
		return
	}

	// Update build with image information and status to built
	if err := s.store.UpdateBuildWithImage(ctx, req.CommitHash, types.BuildStatusBuilt, deployment.ImageTag, deployment.ImageID, deployment.Size); err != nil {
		s.logger.Error("Failed to update build status to built", "error", err)
	}

	s.logger.Info("Build completed successfully", "app_name", req.AppName, "temp_dir", bundle.GetTempDir())

	// Clean up the bundle
	if err := bundle.Cleanup(); err != nil {
		s.logger.Warn("Failed to cleanup bundle", "app_name", req.AppName, "error", err)
	}

	c.JSON(http.StatusCreated, deployment)
}

// listBuildsHandler handles build listing requests
func (s *BaseEngine) listBuildsHandler(c *gin.Context) {
	commitHash := c.Query("commit_hash")

	var builds []*types.Build
	var err error

	if commitHash != "" {
		// Get builds by commit hash
		builds, err = s.store.ListBuildsByCommitHash(c.Request.Context(), commitHash)
	} else {
		// Get all builds
		builds, err = s.store.ListBuilds(c.Request.Context())
	}

	if err != nil {
		s.logger.Error("Failed to list builds", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list builds",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"builds": builds,
		"count":  len(builds),
	})
}

// deleteBuildsHandler handles build deletion requests
func (s *BaseEngine) deleteBuildsHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Build ID is required",
		})
		return
	}

	deletedKeys, count, err := s.store.DeleteBuilds(c.Request.Context(), id)
	if err != nil {
		s.logger.Error("Failed to delete builds", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete builds",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deleted": deletedKeys,
		"count":   count,
	})
}

// SetDockerClient sets the Docker client
func (s *BaseEngine) SetDockerClient(cli *client.Client) {
	s.dockerClient = cli
}

// GetDockerClient returns the Docker client
func (s *BaseEngine) GetDockerClient() *client.Client {
	return s.dockerClient
}

// loggerMiddleware adds logging middleware to Gin
func loggerMiddleware(log *logger.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		log.Info("HTTP Request",
			"method", param.Method,
			"path", param.Path,
			"status", param.StatusCode,
			"latency", param.Latency,
			"client_ip", param.ClientIP,
			"user_agent", param.Request.UserAgent(),
		)
		return ""
	})
}
