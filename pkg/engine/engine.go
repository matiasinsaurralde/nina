// Package engine provides HTTP Engine server functionality for the Nina application.
package engine

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

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
}

// BaseEngine implements the Engine interface
type BaseEngine struct {
	config  *config.Config
	logger  *logger.Logger
	store   *store.Store
	builder builder.Builder
	router  *gin.Engine
	server  *http.Server
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

	// Initialize builder
	b := &builder.BaseBuilder{}
	if err := b.Init(context.Background(), cfg, log); err != nil {
		log.Error("Failed to initialize builder", "error", err)
		// Continue without builder for now
	}

	server := &BaseEngine{
		config:  cfg,
		logger:  log,
		store:   st,
		builder: b,
		router:  router,
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
	v1.POST("/build", s.buildHandler)
	v1.GET("/builds", s.listBuildsHandler)
	v1.DELETE("/builds/:id", s.deleteBuildsHandler) // <-- new endpoint
	v1.DELETE("/deployments/:id", s.deleteDeploymentHandler)
	v1.GET("/deployments/:id/status", s.getDeploymentStatusHandler)
	v1.GET("/deployments", s.listDeploymentsHandler)
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

// deleteDeploymentHandler handles deployment deletion requests
func (s *BaseEngine) deleteDeploymentHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Deployment ID is required",
		})
		return
	}

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
	deployments, err := s.store.ListDeployments(c.Request.Context())
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

// deleteBuildsHandler handles build deletion by app name or commit hash
func (s *BaseEngine) deleteBuildsHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is required"})
		return
	}

	ctx := c.Request.Context()
	builds, err := s.store.ListBuilds(ctx)
	if err != nil {
		s.logger.Error("Failed to list builds for deletion", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list builds"})
		return
	}

	s.logger.Info("Searching for builds to delete", "search_id", id, "total_builds", len(builds))

	toDelete := make([]string, 0)
	keySet := make(map[string]struct{})

	for _, build := range builds {
		key := "nina-build-" + build.CommitHash
		s.logger.Debug("Checking build", "app_name", build.AppName, "commit_hash", build.CommitHash, "search_id", id)
		// Match by exact app name or by commit hash prefix
		if build.AppName == id || strings.HasPrefix(build.CommitHash, id) {
			if _, exists := keySet[key]; !exists {
				toDelete = append(toDelete, key)
				keySet[key] = struct{}{}
				s.logger.Info("Added build to deletion list", "key", key, "app_name", build.AppName, "commit_hash", build.CommitHash)
			}
		}
	}

	s.logger.Info("Builds to delete", "count", len(toDelete), "keys", toDelete)

	deleted := make([]string, 0)
	for _, key := range toDelete {
		// TODO: Docker cleanup for this build (stub)
		// e.g., remove Docker image associated with this build
		if err := s.store.DeleteBuildByKey(ctx, key); err != nil {
			s.logger.Warn("Failed to delete build key", "key", key, "error", err)
			continue
		}
		deleted = append(deleted, key)
	}

	c.JSON(http.StatusOK, gin.H{
		"deleted": deleted,
		"count":   len(deleted),
	})
}

// loggerMiddleware adds logging to requests
func loggerMiddleware(log *logger.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		log.Info("HTTP Request",
			"method", param.Method,
			"path", param.Path,
			"status", param.StatusCode,
			"latency", param.Latency,
			"client_ip", param.ClientIP,
		)
		return ""
	})
}
