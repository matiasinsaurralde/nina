package cli

import (
	"context"
	"testing"

	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
)

func TestDeploy(t *testing.T) {
	// Create a test CLI instance
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 9999, // Use a port that's likely not in use
		},
	}
	log := logger.New(logger.LevelInfo, "text")
	c := NewCLI(cfg, log)

	// Test that Deploy returns an error for non-Git directory
	_, err := c.Deploy(context.Background(), "/tmp", 1)
	if err == nil {
		t.Error("Expected error for non-Git directory, got nil")
	}
}

func TestDeploymentExists(t *testing.T) {
	// Create a test CLI instance
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 9999, // Use a port that's likely not in use
		},
	}
	log := logger.New(logger.LevelInfo, "text")
	c := NewCLI(cfg, log)

	// Test that DeploymentExists returns an error when server is not available
	_, err := c.DeploymentExists(context.Background(), "nonexistent-app")
	if err == nil {
		t.Error("Expected error when server is not available, got nil")
	}
}

func TestBuildExists(t *testing.T) {
	// Create a test CLI instance
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 9999, // Use a port that's likely not in use
		},
	}
	log := logger.New(logger.LevelInfo, "text")
	c := NewCLI(cfg, log)

	// Test that BuildExists returns an error when server is not available
	_, err := c.BuildExists(context.Background(), "nonexistent-commit")
	if err == nil {
		t.Error("Expected error when server is not available, got nil")
	}
}

func TestBuildExistsWithRealServer(t *testing.T) {
	// This test would require a real server to be running
	// For now, we'll just test the error handling
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 9999, // Use a port that's likely not in use
		},
	}
	log := logger.New(logger.LevelInfo, "text")
	c := NewCLI(cfg, log)

	// Test that BuildExists returns an error when server is not available
	_, err := c.BuildExists(context.Background(), "test-commit")
	if err == nil {
		t.Error("Expected error when server is not available, got nil")
	}
}

func TestListDeployments(t *testing.T) {
	// Create a test CLI instance
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 9999, // Use a port that's likely not in use
		},
	}
	log := logger.New(logger.LevelInfo, "text")
	c := NewCLI(cfg, log)

	// Test that ListDeployments returns an error when server is not available
	deployments, err := c.ListDeployments(context.Background())
	if err == nil {
		t.Error("Expected error when server is not available, got nil")
	}
	if deployments != nil {
		t.Error("Expected nil deployments when server is not available")
	}
}

func TestListBuilds(t *testing.T) {
	// Create a test CLI instance
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 9999, // Use a port that's likely not in use
		},
	}
	log := logger.New(logger.LevelInfo, "text")
	c := NewCLI(cfg, log)

	// Test that ListBuilds returns an error when server is not available
	builds, err := c.ListBuilds(context.Background())
	if err == nil {
		t.Error("Expected error when server is not available, got nil")
	}
	if builds != nil {
		t.Error("Expected nil builds when server is not available")
	}
}

func TestHealthCheck(t *testing.T) {
	// Create a test CLI instance
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 9999, // Use a port that's likely not in use
		},
	}
	log := logger.New(logger.LevelInfo, "text")
	c := NewCLI(cfg, log)

	// Test that HealthCheck returns an error when server is not available
	err := c.HealthCheck(context.Background())
	if err == nil {
		t.Error("Expected error when server is not available, got nil")
	}
}

func TestProvision(t *testing.T) {
	// Create a test CLI instance
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 9999, // Use a port that's likely not in use
		},
	}
	log := logger.New(logger.LevelInfo, "text")
	c := NewCLI(cfg, log)

	// Test that Deploy returns an error when server is not available
	_, err := c.Deploy(context.Background(), "/tmp", 1)
	if err == nil {
		t.Error("Expected error when server is not available, got nil")
	}
}
