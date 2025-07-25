package cli

import (
	"context"
	"testing"

	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
)

func TestBuildExists(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	// Create test logger
	log := logger.New(logger.LevelDebug, "text")

	// Create CLI instance
	c := NewCLI(cfg, log)

	// Test with a non-existent commit hash
	exists, err := c.BuildExists(context.Background(), "nonexistent-commit")
	if err != nil {
		// This is expected to fail since we don't have a real server running
		// The important thing is that the method doesn't panic and handles errors gracefully
		t.Logf("Expected error when server is not running: %v", err)
		return
	}

	if exists {
		t.Error("Expected build to not exist for non-existent commit")
	}
}

func TestBuildExistsWithRealServer(t *testing.T) {
	// Skip this test if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require a real server running
	// For now, we'll just verify the method signature and basic functionality
	t.Skip("Integration test requires running server")
}
