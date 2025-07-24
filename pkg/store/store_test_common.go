package store

import (
	"context"
	"testing"
)

// runStoreTestSuite runs the common test suite for both unit and integration tests
func runStoreTestSuite(t *testing.T, store *Store) {
	t.Helper()
	runCreateDeploymentTest(t, store)
	runGetDeploymentTest(t, store)
	runGetDeploymentByNameTest(t, store)
	runUpdateDeploymentStatusTest(t, store)
	runListDeploymentsTest(t, store)
	runDeleteDeploymentTest(t, store)
}

func runCreateDeploymentTest(t *testing.T, store *Store) {
	t.Helper()
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
		if deleteErr := store.DeleteDeployment(context.Background(), deployment.ID); deleteErr != nil {
			t.Errorf("Failed to clean up deployment: %v", deleteErr)
		}
	})
}

func runGetDeploymentTest(t *testing.T, store *Store) {
	t.Helper()
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
		if deleteErr := store.DeleteDeployment(context.Background(), deployment.ID); deleteErr != nil {
			t.Errorf("Failed to clean up deployment: %v", deleteErr)
		}
	})
}

func runGetDeploymentByNameTest(t *testing.T, store *Store) {
	t.Helper()
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
		if deleteErr := store.DeleteDeployment(context.Background(), deployment.ID); deleteErr != nil {
			t.Errorf("Failed to clean up deployment: %v", deleteErr)
		}
	})
}

func runUpdateDeploymentStatusTest(t *testing.T, store *Store) {
	t.Helper()
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
		if updateErr := store.UpdateDeploymentStatus(context.Background(), deployment.ID, "running"); updateErr != nil {
			t.Fatalf("Failed to update deployment status: %v", updateErr)
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
		if deleteErr := store.DeleteDeployment(context.Background(), deployment.ID); deleteErr != nil {
			t.Errorf("Failed to clean up deployment: %v", deleteErr)
		}
	})
}

func runListDeploymentsTest(t *testing.T, store *Store) {
	t.Helper()
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
			if deleteErr := store.DeleteDeployment(context.Background(), id); deleteErr != nil {
				t.Errorf("Failed to clean up deployment %s: %v", id, deleteErr)
			}
		}
	})
}

func runDeleteDeploymentTest(t *testing.T, store *Store) {
	t.Helper()
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
		if deleteErr := store.DeleteDeployment(context.Background(), deployment.ID); deleteErr != nil {
			t.Fatalf("Failed to delete deployment: %v", deleteErr)
		}

		// Try to get it - should fail
		_, err = store.GetDeployment(context.Background(), deployment.ID)
		if err == nil {
			t.Error("Expected error when getting deleted deployment, got nil")
		}
	})
}
