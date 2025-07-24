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

	// Create store
	st, err := NewStore(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer st.Close()

	// Test creating a deployment
	t.Run("CreateDeployment", func(t *testing.T) {
		req := &ProvisionRequest{
			Name:  "test-app",
			Image: "nginx:latest",
			Ports: []int{80, 443},
			Environment: map[string]string{
				"ENV": "test",
			},
		}

		deployment, err := st.CreateDeployment(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to create deployment: %v", err)
		}

		if deployment.Name != req.Name {
			t.Errorf("Expected name %s, got %s", req.Name, deployment.Name)
		}

		if deployment.Image != req.Image {
			t.Errorf("Expected image %s, got %s", req.Image, deployment.Image)
		}

		if deployment.Status != "creating" {
			t.Errorf("Expected status 'creating', got %s", deployment.Status)
		}

		// Clean up
		if err := st.DeleteDeployment(context.Background(), deployment.ID); err != nil {
			t.Errorf("Failed to clean up deployment: %v", err)
		}
	})

	// Test getting a deployment
	t.Run("GetDeployment", func(t *testing.T) {
		req := &ProvisionRequest{
			Name:  "test-get-app",
			Image: "alpine:latest",
			Ports: []int{8080},
		}

		deployment, err := st.CreateDeployment(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to create deployment: %v", err)
		}

		// Get the deployment
		retrieved, err := st.GetDeployment(context.Background(), deployment.ID)
		if err != nil {
			t.Fatalf("Failed to get deployment: %v", err)
		}

		if retrieved.ID != deployment.ID {
			t.Errorf("Expected ID %s, got %s", deployment.ID, retrieved.ID)
		}

		if retrieved.Name != deployment.Name {
			t.Errorf("Expected name %s, got %s", deployment.Name, retrieved.Name)
		}

		// Clean up
		if err := st.DeleteDeployment(context.Background(), deployment.ID); err != nil {
			t.Errorf("Failed to clean up deployment: %v", err)
		}
	})

	// Test getting deployment by name
	t.Run("GetDeploymentByName", func(t *testing.T) {
		req := &ProvisionRequest{
			Name:  "test-name-app",
			Image: "busybox:latest",
			Ports: []int{9000},
		}

		deployment, err := st.CreateDeployment(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to create deployment: %v", err)
		}

		// Get by name
		retrieved, err := st.GetDeploymentByName(context.Background(), req.Name)
		if err != nil {
			t.Fatalf("Failed to get deployment by name: %v", err)
		}

		if retrieved.ID != deployment.ID {
			t.Errorf("Expected ID %s, got %s", deployment.ID, retrieved.ID)
		}

		// Clean up
		if err := st.DeleteDeployment(context.Background(), deployment.ID); err != nil {
			t.Errorf("Failed to clean up deployment: %v", err)
		}
	})

	// Test updating deployment status
	t.Run("UpdateDeploymentStatus", func(t *testing.T) {
		req := &ProvisionRequest{
			Name:  "test-status-app",
			Image: "redis:alpine",
			Ports: []int{6379},
		}

		deployment, err := st.CreateDeployment(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to create deployment: %v", err)
		}

		// Update status
		if err := st.UpdateDeploymentStatus(context.Background(), deployment.ID, "running"); err != nil {
			t.Fatalf("Failed to update deployment status: %v", err)
		}

		// Get and verify
		retrieved, err := st.GetDeployment(context.Background(), deployment.ID)
		if err != nil {
			t.Fatalf("Failed to get deployment: %v", err)
		}

		if retrieved.Status != "running" {
			t.Errorf("Expected status 'running', got %s", retrieved.Status)
		}

		// Clean up
		if err := st.DeleteDeployment(context.Background(), deployment.ID); err != nil {
			t.Errorf("Failed to clean up deployment: %v", err)
		}
	})

	// Test listing deployments
	t.Run("ListDeployments", func(t *testing.T) {
		// Create multiple deployments
		deployments := []*ProvisionRequest{
			{Name: "list-app-1", Image: "nginx:latest", Ports: []int{80}},
			{Name: "list-app-2", Image: "alpine:latest", Ports: []int{8080}},
			{Name: "list-app-3", Image: "busybox:latest", Ports: []int{9000}},
		}

		createdIDs := make([]string, 0, len(deployments))
		for _, req := range deployments {
			deployment, err := st.CreateDeployment(context.Background(), req)
			if err != nil {
				t.Fatalf("Failed to create deployment: %v", err)
			}
			createdIDs = append(createdIDs, deployment.ID)
		}

		// List deployments
		list, err := st.ListDeployments(context.Background())
		if err != nil {
			t.Fatalf("Failed to list deployments: %v", err)
		}

		// Should have at least our test deployments
		if len(list) < len(deployments) {
			t.Errorf("Expected at least %d deployments, got %d", len(deployments), len(list))
		}

		// Clean up
		for _, id := range createdIDs {
			if err := st.DeleteDeployment(context.Background(), id); err != nil {
				t.Errorf("Failed to clean up deployment %s: %v", id, err)
			}
		}
	})

	// Test deleting deployment
	t.Run("DeleteDeployment", func(t *testing.T) {
		req := &ProvisionRequest{
			Name:  "test-delete-app",
			Image: "nginx:latest",
			Ports: []int{80},
		}

		deployment, err := st.CreateDeployment(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to create deployment: %v", err)
		}

		// Delete the deployment
		if err := st.DeleteDeployment(context.Background(), deployment.ID); err != nil {
			t.Fatalf("Failed to delete deployment: %v", err)
		}

		// Try to get it - should fail
		_, err = st.GetDeployment(context.Background(), deployment.ID)
		if err == nil {
			t.Error("Expected error when getting deleted deployment, got nil")
		}
	})
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

	// Create store
	st, err := NewStore(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer st.Close()

	// Test concurrent deployment creation
	t.Run("ConcurrentDeploymentCreation", func(t *testing.T) {
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
	})
}
