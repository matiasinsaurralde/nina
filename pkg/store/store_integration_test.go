//go:build integration
// +build integration

package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
)

func TestStoreIntegration(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test configuration
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       1, // Use different DB for tests
		},
	}

	// Create test logger
	log := logger.New(logger.LevelDebug, "text")

	// Create mock store (will use real Redis if available, otherwise Miniredis)
	st, err := NewMockStore(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create mock store: %v", err)
	}
	defer func() {
		if closeErr := st.Close(); closeErr != nil {
			t.Logf("Failed to close store: %v", closeErr)
		}
	}()

	// Clear any existing data
	if err := st.FlushAll(context.Background()); err != nil {
		t.Fatalf("Failed to flush store: %v", err)
	}

	// Run the same test suite as unit tests but with real store
	runStoreTestSuite(t, st.Store)
}

func TestStoreConcurrency(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test configuration
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       2, // Use different DB for concurrency tests
		},
	}

	// Create test logger
	log := logger.New(logger.LevelDebug, "text")

	// Create mock store (will use real Redis if available, otherwise Miniredis)
	st, err := NewMockStore(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create mock store: %v", err)
	}
	defer func() {
		if closeErr := st.Close(); closeErr != nil {
			t.Logf("Failed to close store: %v", closeErr)
		}
	}()

	// Clear any existing data
	if err := st.FlushAll(context.Background()); err != nil {
		t.Fatalf("Failed to flush store: %v", err)
	}

	// Test concurrent deployment creation
	t.Run("ConcurrentDeploymentCreation", func(t *testing.T) {
		runConcurrencyTest(t, st)
	})
}

func runConcurrencyTest(t *testing.T, st *MockStore) {
	t.Helper()
	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			req := &ProvisionRequest{
				Name:  fmt.Sprintf("concurrent-app-%d", id),
				Image: "nginx:latest",
				Ports: []int{80 + id},
			}

			deployment, err := st.CreateDeployment(context.Background(), req)
			if err != nil {
				results <- fmt.Errorf("failed to create deployment %d: %v", id, err)
				return
			}

			// Clean up
			if err := st.DeleteDeployment(context.Background(), deployment.ID); err != nil {
				results <- fmt.Errorf("failed to delete deployment %d: %v", id, err)
				return
			}

			results <- nil
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	}
}
