// Package engine provides HTTP Engine server functionality for the Nina application.
package engine

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"reflect"
	"strconv"
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

// validateDeploymentRequest validates the deployment request
func (s *BaseEngine) validateDeploymentRequest(req *types.DeploymentRequest) error {
	if req.AppName == "" || req.CommitHash == "" {
		return fmt.Errorf("app name and commit hash are required")
	}
	return nil
}

// validateBuildForDeployment validates that the build exists and is ready for deployment
func (s *BaseEngine) validateBuildForDeployment(ctx context.Context, commitHash string) (*types.Build, error) {
	build, err := s.store.GetBuild(ctx, commitHash)
	if err != nil {
		return nil, fmt.Errorf("build not found for the given commit hash: %w", err)
	}

	if build.Status != types.BuildStatusBuilt {
		return nil, fmt.Errorf("build is not ready for deployment (status: %s)", build.Status)
	}

	return build, nil
}

// createDeploymentRecord creates a deployment record in the store
func (s *BaseEngine) createDeploymentRecord(ctx context.Context, req *types.DeploymentRequest) (*types.Deployment, error) {
	deployment, err := s.store.CreateNewDeployment(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment record: %w", err)
	}

	// Update deployment status to deploying
	if err := s.store.UpdateNewDeploymentStatus(ctx, req.AppName, types.DeploymentStatusDeploying); err != nil {
		s.logger.Error("Failed to update deployment status to deploying", "error", err)
	}

	return deployment, nil
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
	if err := s.validateDeploymentRequest(&req); err != nil {
		s.logger.Error("Invalid deployment request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	s.logger.Info("Processing deployment request", "app_name", req.AppName, "commit_hash", req.CommitHash, "replicas", req.Replicas)

	// Validate build
	build, err := s.validateBuildForDeployment(ctx, req.CommitHash)
	if err != nil {
		s.logger.Error("Build validation failed", "commit_hash", req.CommitHash, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Create deployment record
	deployment, err := s.createDeploymentRecord(ctx, &req)
	if err != nil {
		s.logger.Error("Failed to create deployment record", "app_name", req.AppName, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Deploy containers in background
	go func() {
		s.logger.Info("Starting container deployment in background", "app_name", req.AppName, "replicas", req.Replicas)
		if err := s.deployContainers(context.Background(), req.AppName, build.ImageTag, req.Replicas); err != nil {
			s.logger.Error("Failed to deploy containers", "app_name", req.AppName, "error", err)
			if updateErr := s.store.UpdateNewDeploymentStatus(context.Background(), req.AppName, types.DeploymentStatusFailed); updateErr != nil {
				s.logger.Error("Failed to update deployment status to failed", "error", updateErr)
			}
		}
	}()

	c.JSON(http.StatusCreated, deployment)
}

// createContainerConfig creates the container configuration
func (s *BaseEngine) createContainerConfig(imageTag string, containerPort int) *container.Config {
	return &container.Config{
		Image: imageTag,
		Env: []string{
			fmt.Sprintf("PORT=%d", containerPort),
		},
		ExposedPorts: nat.PortSet{
			nat.Port(fmt.Sprintf("%d/tcp", containerPort)): struct{}{},
		},
	}
}

// createHostConfig creates the host configuration for port binding
func (s *BaseEngine) createHostConfig(containerPort int) *container.HostConfig {
	return &container.HostConfig{
		PortBindings: nat.PortMap{
			nat.Port(fmt.Sprintf("%d/tcp", containerPort)): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "", // Empty string = Docker assigns random available port
				},
			},
		},
	}
}

// createAndStartContainer creates and starts a single container
func (s *BaseEngine) createAndStartContainer(
	ctx context.Context,
	appName, imageTag string,
	containerPort, replica int,
) (*types.Container, error) {
	s.logger.Info("Creating container", "replica", replica, "app_name", appName)

	containerConfig := s.createContainerConfig(imageTag, containerPort)
	hostConfig := s.createHostConfig(containerPort)

	// Create container with unique name
	containerName := s.generateUniqueContainerName(appName, replica)
	resp, err := s.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create container %d: %w", replica, err)
	}

	containerID := resp.ID
	s.logger.Info("Container created", "container_id", containerID, "app_name", appName, "replica", replica)

	// Start container
	if startErr := s.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{}); startErr != nil {
		return nil, fmt.Errorf("failed to start container %d: %w", replica, startErr)
	}

	// Get the actual assigned host port by inspecting the container
	containerInfo, err := s.dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %d: %w", replica, err)
	}

	// Extract the assigned host port
	var hostPort int
	if bindings, exists := containerInfo.NetworkSettings.Ports[nat.Port(fmt.Sprintf("%d/tcp", containerPort))]; exists && len(bindings) > 0 {
		hostPort, _ = strconv.Atoi(bindings[0].HostPort)
		s.logger.Info("Container port mapping", "container_id", containerID, "container_port", containerPort,
			"host_port", hostPort, "replica", replica)
	} else {
		return nil, fmt.Errorf("failed to get assigned host port for container %s", containerID)
	}

	s.logger.Info("Container started", "container_id", containerID, "app_name", appName, "host_port", hostPort, "replica", replica)

	// Create container info with the actual assigned port
	containerData := &types.Container{
		ContainerID: containerID,
		ImageTag:    imageTag,
		Address:     "localhost",
		Port:        hostPort, // Use the actual assigned host port
	}

	return containerData, nil
}

// deployContainers deploys containers for the given app
func (s *BaseEngine) deployContainers(ctx context.Context, appName, imageTag string, replicas int) error {
	s.logger.Info("Starting container deployment", "app_name", appName, "image_tag", imageTag, "replicas", replicas)

	// Use Docker's automatic port assignment to avoid conflicts
	containerPort := 8080 // Default container port (from Dockerfile)

	var containers []types.Container

	// Create multiple containers based on replicas count
	for i := 0; i < replicas; i++ {
		containerData, err := s.createAndStartContainer(ctx, appName, imageTag, containerPort, i+1)
		if err != nil {
			return err
		}

		containers = append(containers, *containerData)
		s.logger.Info("Container added to list", "replica", i+1, "total_containers", len(containers))
	}

	// Update deployment with all container information and set status to ready
	if err := s.store.UpdateNewDeploymentWithContainers(ctx, appName, containers, types.DeploymentStatusReady); err != nil {
		return fmt.Errorf("failed to update deployment with containers: %w", err)
	}

	s.logger.Info("Deployment completed successfully", "app_name", appName, "replicas", replicas, "containers", len(containers))
	return nil
}

// generateUniqueContainerName generates a unique container name
func (s *BaseEngine) generateUniqueContainerName(appName string, replica int) string {
	// Generate a random number for uniqueness
	n, _ := rand.Int(rand.Reader, big.NewInt(999999))
	return fmt.Sprintf("nina-%s-%d-%d", appName, replica, n.Int64())
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
	containersRemoved := 0
	for _, cont := range deployment.Containers {
		if cont.ContainerID != "" {
			s.logger.Info("Removing container", "container_id", cont.ContainerID, "app_name", deployment.AppName, "port", cont.Port)
			if err := s.dockerClient.ContainerRemove(c.Request.Context(), cont.ContainerID, container.RemoveOptions{Force: true}); err != nil {
				s.logger.Error("Failed to remove container", "container_id", cont.ContainerID, "error", err)
				// Continue with other containers even if one fails
			} else {
				containersRemoved++
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

	s.logger.Info("Deployment deleted successfully", "id", id, "app_name", deployment.AppName, "containers_removed", containersRemoved)
	c.JSON(http.StatusOK, gin.H{
		"message":            "Deployment deleted successfully",
		"id":                 id,
		"containers_removed": containersRemoved,
	})
}

// getDeploymentWrapper wraps the store.GetDeployment function to match the interface
func (s *BaseEngine) getDeploymentWrapper(ctx context.Context, id string) (interface{}, error) {
	deployment, err := s.store.GetDeployment(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}
	return deployment, nil
}

// getDeploymentHandler handles deployment retrieval requests
func (s *BaseEngine) getDeploymentHandler(c *gin.Context) {
	s.handleGetByID(c, s.getDeploymentWrapper, "deployment")
}

// getDeploymentStatusHandler handles deployment status requests
func (s *BaseEngine) getDeploymentStatusHandler(c *gin.Context) {
	s.handleGetByID(c, s.getDeploymentWrapper, "deployment")
}

// listDeploymentsWrapper wraps the store.ListNewDeployments function
func (s *BaseEngine) listDeploymentsWrapper(ctx context.Context) (interface{}, error) {
	deployments, err := s.store.ListNewDeployments(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	return deployments, nil
}

// listDeploymentsByAppNameWrapper wraps the store.ListNewDeploymentsByAppName function
func (s *BaseEngine) listDeploymentsByAppNameWrapper(ctx context.Context, appName string) (interface{}, error) {
	deployments, err := s.store.ListNewDeploymentsByAppName(ctx, appName)
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments by app name: %w", err)
	}
	return deployments, nil
}

// listDeploymentsHandler handles deployment listing requests
func (s *BaseEngine) listDeploymentsHandler(c *gin.Context) {
	s.handleList(c, s.listDeploymentsWrapper, s.listDeploymentsByAppNameWrapper, "app_name", "deployments")
}

// validateBuildRequest validates the build request
func (s *BaseEngine) validateBuildRequest(req *types.BuildRequest) error {
	if req.AppName == "" || req.BundleContents == "" {
		return fmt.Errorf("app name and bundle contents are required")
	}
	return nil
}

// createBuildRecord creates a build record in the store
func (s *BaseEngine) createBuildRecord(ctx context.Context, req *types.BuildRequest) error {
	_, err := s.store.CreateBuild(ctx, req)
	if err != nil {
		s.logger.Error("Failed to create build record", "app_name", req.AppName, "error", err)
		return fmt.Errorf("failed to create build record: %w", err)
	}
	return nil
}

// extractAndMatchBundle extracts the bundle and matches it with a buildpack
func (s *BaseEngine) extractAndMatchBundle(ctx context.Context, req *types.BuildRequest) (*builder.Bundle, builder.Buildpack, error) {
	// Extract bundle
	bundle, err := s.builder.ExtractBundle(ctx, req)
	if err != nil {
		s.logger.Error("Failed to extract bundle", "app_name", req.AppName, "error", err)
		// Update build status to failed
		if updateErr := s.store.UpdateBuildStatus(ctx, req.CommitHash, types.BuildStatusFailed); updateErr != nil {
			s.logger.Error("Failed to update build status to failed", "error", updateErr)
		}
		return nil, nil, fmt.Errorf("failed to extract bundle: %w", err)
	}

	// Match buildpack
	buildpack, err := s.builder.MatchBuildpack(ctx, req)
	if err != nil {
		s.logger.Error("Failed to match buildpack", "app_name", req.AppName, "error", err)
		// Update build status to failed
		if updateErr := s.store.UpdateBuildStatus(ctx, req.CommitHash, types.BuildStatusFailed); updateErr != nil {
			s.logger.Error("Failed to update build status to failed", "error", updateErr)
		}
		return nil, nil, fmt.Errorf("failed to match buildpack: %w", err)
	}

	if buildpack == nil {
		s.logger.Warn("No matching buildpack found", "app_name", req.AppName)
		// Update build status to failed
		if updateErr := s.store.UpdateBuildStatus(ctx, req.CommitHash, types.BuildStatusFailed); updateErr != nil {
			s.logger.Error("Failed to update build status to failed", "error", updateErr)
		}
		return nil, nil, fmt.Errorf("no matching buildpack found for this project type")
	}

	s.logger.Info("Buildpack matched", "app_name", req.AppName, "buildpack", buildpack.Name())
	return bundle, buildpack, nil
}

// buildProject builds the project using the matched buildpack
func (s *BaseEngine) buildProject(
	ctx context.Context,
	req *types.BuildRequest,
	bundle *builder.Bundle,
	buildpack builder.Buildpack,
) (*types.DeploymentImage, error) {
	// Update build status to building
	if updateErr := s.store.UpdateBuildStatus(ctx, req.CommitHash, types.BuildStatusBuilding); updateErr != nil {
		s.logger.Error("Failed to update build status to building", "error", updateErr)
	}

	// Build the project
	deployment, err := buildpack.Build(ctx, bundle)
	if err != nil {
		s.logger.Error("Failed to build project", "app_name", req.AppName, "error", err)
		// Update build status to failed
		if updateErr := s.store.UpdateBuildStatus(ctx, req.CommitHash, types.BuildStatusFailed); updateErr != nil {
			s.logger.Error("Failed to update build status to failed", "error", updateErr)
		}
		return nil, fmt.Errorf("failed to build project: %w", err)
	}

	// Update build with image information and status to built
	if err := s.store.UpdateBuildWithImage(ctx, req.CommitHash, types.BuildStatusBuilt, deployment.ImageTag,
		deployment.ImageID, deployment.Size); err != nil {
		s.logger.Error("Failed to update build status to built", "error", err)
	}

	s.logger.Info("Build completed successfully", "app_name", req.AppName, "temp_dir", bundle.GetTempDir())

	// Clean up the bundle
	if err := bundle.Cleanup(); err != nil {
		s.logger.Warn("Failed to cleanup bundle", "app_name", req.AppName, "error", err)
	}

	return deployment, nil
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
	if err := s.validateBuildRequest(&req); err != nil {
		s.logger.Error("Invalid build request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	s.logger.Info("Processing build request", "app_name", req.AppName, "commit_hash", req.CommitHash)

	// Create build record
	if err := s.createBuildRecord(ctx, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Extract bundle and match buildpack
	bundle, buildpack, err := s.extractAndMatchBundle(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Build the project
	deployment, err := s.buildProject(ctx, &req, bundle, buildpack)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, deployment)
}

// listBuildsWrapper wraps the store.ListBuilds function
func (s *BaseEngine) listBuildsWrapper(ctx context.Context) (interface{}, error) {
	builds, err := s.store.ListBuilds(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list builds: %w", err)
	}
	return builds, nil
}

// listBuildsByCommitHashWrapper wraps the store.ListBuildsByCommitHash function
func (s *BaseEngine) listBuildsByCommitHashWrapper(ctx context.Context, commitHash string) (interface{}, error) {
	builds, err := s.store.ListBuildsByCommitHash(ctx, commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to list builds by commit hash: %w", err)
	}
	return builds, nil
}

// listBuildsHandler handles build listing requests
func (s *BaseEngine) listBuildsHandler(c *gin.Context) {
	s.handleList(c, s.listBuildsWrapper, s.listBuildsByCommitHashWrapper, "commit_hash", "builds")
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

// handleGetByID is a helper function to handle GET requests by ID
func (s *BaseEngine) handleGetByID(c *gin.Context, getFunc func(context.Context, string) (interface{}, error), idType string) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("%s ID is required", idType),
		})
		return
	}

	item, err := getFunc(c.Request.Context(), id)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to get %s", idType), "id", id, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("%s not found", idType),
		})
		return
	}

	c.JSON(http.StatusOK, item)
}

// handleList is a helper function to handle list requests
func (s *BaseEngine) handleList(
	c *gin.Context,
	listAllFunc func(context.Context) (interface{}, error),
	listByFunc func(context.Context, string) (interface{}, error),
	queryParam, itemType string,
) {
	query := c.Query(queryParam)

	var items interface{}
	var err error

	if query != "" {
		// Get items by query parameter
		items, err = listByFunc(c.Request.Context(), query)
	} else {
		// Get all items
		items, err = listAllFunc(c.Request.Context())
	}

	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to list %s", itemType), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to list %s", itemType),
		})
		return
	}

	// Use reflection to get the length of the slice
	itemsValue := reflect.ValueOf(items)
	if itemsValue.Kind() == reflect.Slice {
		c.JSON(http.StatusOK, gin.H{
			itemType: items,
			"count":  itemsValue.Len(),
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			itemType: items,
			"count":  0,
		})
	}
}
