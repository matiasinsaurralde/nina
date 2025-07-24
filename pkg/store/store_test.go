package store

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/redis/go-redis/v9"
)

func TestStoreWithMiniredis(t *testing.T) {
	// Start Miniredis
	mockRedis, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start Miniredis: %v", err)
	}
	defer mockRedis.Close()

	// Create test configuration
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     mockRedis.Host(),
			Port:     mockRedis.Server().Addr().Port,
			Password: "",
			DB:       0,
		},
	}

	// Create test logger
	log := logger.New(logger.LevelDebug, "text")

	// Create store with mock Redis
	client := redis.NewClient(&redis.Options{
		Addr: mockRedis.Addr(),
	})

	store := &Store{
		client: client,
		logger: log,
		config: cfg,
	}

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

		deployment, err := store.CreateDeployment(context.Background(), req)
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
		if err := store.DeleteDeployment(context.Background(), deployment.ID); err != nil {
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

		deployment, err := store.CreateDeployment(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to create deployment: %v", err)
		}

		// Get the deployment
		retrieved, err := store.GetDeployment(context.Background(), deployment.ID)
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
		if err := store.DeleteDeployment(context.Background(), deployment.ID); err != nil {
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

		deployment, err := store.CreateDeployment(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to create deployment: %v", err)
		}

		// Get by name
		retrieved, err := store.GetDeploymentByName(context.Background(), req.Name)
		if err != nil {
			t.Fatalf("Failed to get deployment by name: %v", err)
		}

		if retrieved.ID != deployment.ID {
			t.Errorf("Expected ID %s, got %s", deployment.ID, retrieved.ID)
		}

		// Clean up
		if err := store.DeleteDeployment(context.Background(), deployment.ID); err != nil {
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

		deployment, err := store.CreateDeployment(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to create deployment: %v", err)
		}

		// Update status
		if err := store.UpdateDeploymentStatus(context.Background(), deployment.ID, "running"); err != nil {
			t.Fatalf("Failed to update deployment status: %v", err)
		}

		// Get and verify
		retrieved, err := store.GetDeployment(context.Background(), deployment.ID)
		if err != nil {
			t.Fatalf("Failed to get deployment: %v", err)
		}

		if retrieved.Status != "running" {
			t.Errorf("Expected status 'running', got %s", retrieved.Status)
		}

		// Clean up
		if err := store.DeleteDeployment(context.Background(), deployment.ID); err != nil {
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
			deployment, err := store.CreateDeployment(context.Background(), req)
			if err != nil {
				t.Fatalf("Failed to create deployment: %v", err)
			}
			createdIDs = append(createdIDs, deployment.ID)
		}

		// List deployments
		list, err := store.ListDeployments(context.Background())
		if err != nil {
			t.Fatalf("Failed to list deployments: %v", err)
		}

		// Should have at least our test deployments
		if len(list) < len(deployments) {
			t.Errorf("Expected at least %d deployments, got %d", len(deployments), len(list))
		}

		// Clean up
		for _, id := range createdIDs {
			if err := store.DeleteDeployment(context.Background(), id); err != nil {
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

		deployment, err := store.CreateDeployment(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to create deployment: %v", err)
		}

		// Delete the deployment
		if err := store.DeleteDeployment(context.Background(), deployment.ID); err != nil {
			t.Fatalf("Failed to delete deployment: %v", err)
		}

		// Try to get it - should fail
		_, err = store.GetDeployment(context.Background(), deployment.ID)
		if err == nil {
			t.Error("Expected error when getting deleted deployment, got nil")
		}
	})
}
