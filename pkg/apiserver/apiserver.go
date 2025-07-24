// Package apiserver provides HTTP API server functionality for the Nina application.
package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/store"
)

// APIServer defines the interface for the API server
type APIServer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	SetConfig(cfg *config.Config)
	GetConfig() *config.Config
}

// BaseAPIServer implements the APIServer interface
type BaseAPIServer struct {
	config *config.Config
	logger *logger.Logger
	store  *store.Store
	router *gin.Engine
	server *http.Server
}

// NewAPIServer creates a new API server instance
func NewAPIServer(cfg *config.Config, log *logger.Logger, st *store.Store) APIServer {
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

	server := &BaseAPIServer{
		config: cfg,
		logger: log,
		store:  st,
		router: router,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// Start starts the API server
func (s *BaseAPIServer) Start(ctx context.Context) error {
	s.server = &http.Server{
		Addr:              s.config.GetServerAddr(),
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.logger.Info("Starting API server", "addr", s.config.GetServerAddr())

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Failed to start server", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	return s.Stop(context.Background())
}

// Stop stops the API server
func (s *BaseAPIServer) Stop(ctx context.Context) error {
	if s.server != nil {
		s.logger.Info("Stopping API server")
		return fmt.Errorf("failed to shutdown server: %w", s.server.Shutdown(ctx))
	}
	return nil
}

// SetConfig sets the configuration
func (s *BaseAPIServer) SetConfig(cfg *config.Config) {
	s.config = cfg
}

// GetConfig returns the current configuration
func (s *BaseAPIServer) GetConfig() *config.Config {
	return s.config
}

// setupRoutes sets up the API routes
func (s *BaseAPIServer) setupRoutes() {
	// Health check
	s.router.GET("/health", s.healthHandler)

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	v1.POST("/provision", s.provisionHandler)
	v1.DELETE("/deployments/:id", s.deleteDeploymentHandler)
	v1.GET("/deployments/:id/status", s.getDeploymentStatusHandler)
	v1.GET("/deployments", s.listDeploymentsHandler)
}

// healthHandler handles health check requests
func (s *BaseAPIServer) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"service":   "nina-api",
	})
}

// provisionHandler handles container provisioning requests
func (s *BaseAPIServer) provisionHandler(c *gin.Context) {
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
func (s *BaseAPIServer) deleteDeploymentHandler(c *gin.Context) {
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
func (s *BaseAPIServer) getDeploymentStatusHandler(c *gin.Context) {
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
func (s *BaseAPIServer) listDeploymentsHandler(c *gin.Context) {
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
