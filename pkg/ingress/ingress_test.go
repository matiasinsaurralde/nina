package ingress

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/store"
	"github.com/matiasinsaurralde/nina/pkg/types"
)

const (
	testAppName = "app1"
)

func TestIngress_DeploymentsCache(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Ingress: config.IngressConfig{
			Host:                      "localhost",
			Port:                      8081,
			DeploymentRefreshInterval: 1, // 1 second for testing
		},
	}

	// Create logger
	log := logger.New(logger.LevelDebug, "text")

	// Create mock store
	mockStore := &store.Store{}

	// Create ingress
	ingress := NewIngress(cfg, log, mockStore)

	// Test initial state
	deployments := ingress.getDeployments()
	if len(deployments) != 0 {
		t.Errorf("Expected empty deployments cache, got %d", len(deployments))
	}

	// Test concurrent access
	var wg sync.WaitGroup
	numGoroutines := 10
	numReads := 100

	// Start multiple goroutines reading from the cache
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numReads; j++ {
				_ = ingress.getDeployments()
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Test that we can still access the cache after concurrent reads
	deployments = ingress.getDeployments()
	if len(deployments) != 0 {
		t.Errorf("Expected empty deployments cache after concurrent access, got %d", len(deployments))
	}
}

func TestIngress_FindDeploymentByAppName(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Ingress: config.IngressConfig{
			Host:                      "localhost",
			Port:                      8081,
			DeploymentRefreshInterval: 1,
		},
	}

	// Create logger
	log := logger.New(logger.LevelDebug, "text")

	// Create mock store
	mockStore := &store.Store{}

	// Create ingress
	ingress := NewIngress(cfg, log, mockStore)

	// Test with empty deployments
	deployment := ingress.findDeploymentByAppName("test-app")
	if deployment != nil {
		t.Errorf("Expected nil deployment for empty cache, got %v", deployment)
	}

	// Test with populated deployments
	testDeployments := []*types.Deployment{
		{
			ID:      "1",
			AppName: testAppName,
			Containers: []types.Container{
				{ContainerID: "container1", Address: "localhost", Port: 8080},
			},
		},
		{
			ID:      "2",
			AppName: "app2",
			Containers: []types.Container{
				{ContainerID: "container2", Address: "localhost", Port: 8081},
			},
		},
	}

	// Manually set deployments (simulating what fetchDeployments would do)
	ingress.deploymentsMux.Lock()
	ingress.deployments = testDeployments
	ingress.deploymentsMux.Unlock()

	// Test finding existing deployment
	deployment = ingress.findDeploymentByAppName(testAppName)
	if deployment == nil {
		t.Fatalf("Expected to find deployment for '%s', got nil", testAppName)
	}
	if deployment.AppName != testAppName {
		t.Errorf("Expected app name '%s', got '%s'", testAppName, deployment.AppName)
	}

	// Test finding non-existing deployment
	deployment = ingress.findDeploymentByAppName("non-existing")
	if deployment != nil {
		t.Errorf("Expected nil for non-existing app, got %v", deployment)
	}
}

func TestIngress_SelectRandomReplica(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Ingress: config.IngressConfig{
			Host:                      "localhost",
			Port:                      8081,
			DeploymentRefreshInterval: 1,
		},
	}

	// Create logger
	log := logger.New(logger.LevelDebug, "text")

	// Create mock store
	mockStore := &store.Store{}

	// Create ingress
	ingress := NewIngress(cfg, log, mockStore)

	// Test with deployment that has no containers
	deployment := &types.Deployment{
		ID:         "1",
		AppName:    testAppName,
		Containers: []types.Container{},
	}

	container := ingress.selectRandomReplica(deployment)
	if container != nil {
		t.Errorf("Expected nil container for deployment with no containers, got %v", container)
	}

	// Test with deployment that has containers
	deployment = &types.Deployment{
		ID:      "1",
		AppName: testAppName,
		Containers: []types.Container{
			{ContainerID: "container1", Address: "localhost", Port: 8080},
			{ContainerID: "container2", Address: "localhost", Port: 8081},
			{ContainerID: "container3", Address: "localhost", Port: 8082},
		},
	}

	// Test multiple selections to ensure randomness (though this is not deterministic)
	selectedContainers := make(map[string]bool)
	for i := 0; i < 100; i++ {
		container = ingress.selectRandomReplica(deployment)
		if container == nil {
			t.Error("Expected non-nil container, got nil")
			break
		}
		selectedContainers[container.ContainerID] = true
	}

	// We should have selected at least 2 different containers in 100 attempts
	if len(selectedContainers) < 2 {
		t.Errorf("Expected at least 2 different containers selected, got %d", len(selectedContainers))
	}
}

func TestIngress_HandleRequest_UnknownApplication(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Ingress: config.IngressConfig{
			Host:                      "localhost",
			Port:                      8081,
			DeploymentRefreshInterval: 1,
		},
	}

	// Create logger
	log := logger.New(logger.LevelDebug, "text")

	// Create mock store
	mockStore := &store.Store{}

	// Create ingress
	ingress := NewIngress(cfg, log, mockStore)

	// Create test request
	req := httptest.NewRequest("GET", "/", http.NoBody)
	req.Host = "unknown-app"

	// Create response recorder
	w := httptest.NewRecorder()

	// Handle request
	ingress.handleRequest(w, req)

	// Check response
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected content type 'application/json', got '%s'", contentType)
	}

	// Check response body
	var errorResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if errorResp.Error != "unknown_application" {
		t.Errorf("Expected error 'unknown_application', got '%s'", errorResp.Error)
	}

	if errorResp.Message != "unknown application" {
		t.Errorf("Expected message 'unknown application', got '%s'", errorResp.Message)
	}
}

func TestIngress_HandleRequest_NoReplicasAvailable(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Ingress: config.IngressConfig{
			Host:                      "localhost",
			Port:                      8081,
			DeploymentRefreshInterval: 1,
		},
	}

	// Create logger
	log := logger.New(logger.LevelDebug, "text")

	// Create mock store
	mockStore := &store.Store{}

	// Create ingress
	ingress := NewIngress(cfg, log, mockStore)

	// Set up deployment with no containers
	testDeployments := []*types.Deployment{
		{
			ID:         "1",
			AppName:    "app1",
			Containers: []types.Container{},
		},
	}

	ingress.deploymentsMux.Lock()
	ingress.deployments = testDeployments
	ingress.deploymentsMux.Unlock()

	// Create test request
	req := httptest.NewRequest("GET", "/", http.NoBody)
	req.Host = testAppName

	// Create response recorder
	w := httptest.NewRecorder()

	// Handle request
	ingress.handleRequest(w, req)

	// Check response
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected content type 'application/json', got '%s'", contentType)
	}

	// Check response body
	var errorResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if errorResp.Error != "no_replicas_available" {
		t.Errorf("Expected error 'no_replicas_available', got '%s'", errorResp.Error)
	}

	if errorResp.Message != "no replicas available" {
		t.Errorf("Expected message 'no replicas available', got '%s'", errorResp.Message)
	}
}

func TestIngress_HandleRequest_ValidRouting(t *testing.T) { //nolint: funlen
	// Start a real backend server
	backendCalled := false
	var receivedContainerID string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled = true
		receivedContainerID = r.Header.Get("X-Nina-Replica-Container-ID")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello from backend"))
	}))
	defer backend.Close()

	// Parse backend address and port
	backendURL := backend.URL // e.g. http://127.0.0.1:12345
	urlParts := strings.Split(strings.TrimPrefix(backendURL, "http://"), ":")
	if len(urlParts) != 2 {
		t.Fatalf("unexpected backend URL: %s", backendURL)
	}
	backendAddr := urlParts[0]
	backendPort, err := strconv.Atoi(urlParts[1])
	if err != nil {
		t.Fatalf("invalid backend port: %v", err)
	}

	// Create test config
	cfg := &config.Config{
		Ingress: config.IngressConfig{
			Host:                      "localhost",
			Port:                      8081,
			DeploymentRefreshInterval: 1,
		},
	}

	log := logger.New(logger.LevelDebug, "text")
	mockStore := &store.Store{}
	ingress := NewIngress(cfg, log, mockStore)

	containerID := "container1"
	testDeployments := []*types.Deployment{
		{
			ID:      "1",
			AppName: testAppName,
			Containers: []types.Container{
				{ContainerID: containerID, Address: backendAddr, Port: backendPort},
			},
		},
	}
	ingress.deploymentsMux.Lock()
	ingress.deployments = testDeployments
	ingress.deploymentsMux.Unlock()

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Host = testAppName
	req.Header.Set("Host", testAppName)
	w := httptest.NewRecorder()

	ingress.handleRequest(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	if !backendCalled {
		t.Fatal("Expected backend to be called, but it was not")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 from backend, got %d", resp.StatusCode)
	}
	if string(body) != "hello from backend" {
		t.Errorf("Expected backend response body, got: %s", string(body))
	}
	if receivedContainerID != containerID {
		t.Errorf("Expected X-Nina-Replica-Container-ID header to be %q, got %q", containerID, receivedContainerID)
	}
}

func TestIngress_DeploymentFetcher(t *testing.T) {
	t.Skip("Skipping deployment fetcher test - requires proper store setup")

	// Create test config with very short refresh interval
	cfg := &config.Config{
		Ingress: config.IngressConfig{
			Host:                      "localhost",
			Port:                      8081,
			DeploymentRefreshInterval: 1, // 1 second
		},
	}

	// Create logger
	log := logger.New(logger.LevelDebug, "text")

	// Create mock store
	mockStore := &store.Store{}

	// Create ingress
	ingress := NewIngress(cfg, log, mockStore)

	// Test that the fetcher can be started and stopped without panicking
	// Note: This test doesn't actually test the store integration since we're using a mock
	// In a real scenario, the store would be properly initialized with Redis

	// Start the fetcher in a goroutine
	go ingress.deploymentFetcher()

	// Wait a bit for the initial fetch
	time.Sleep(100 * time.Millisecond)

	// Stop the fetcher
	close(ingress.stopChan)

	// Wait for the goroutine to finish
	ingress.wg.Wait()
}

func TestIngress_Stop(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Ingress: config.IngressConfig{
			Host:                      "localhost",
			Port:                      8081,
			DeploymentRefreshInterval: 1,
		},
	}

	// Create logger
	log := logger.New(logger.LevelDebug, "text")

	// Create mock store
	mockStore := &store.Store{}

	// Create ingress
	ingress := NewIngress(cfg, log, mockStore)

	// Test stopping without starting (should not panic)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := ingress.Stop(ctx)
	if err != nil {
		t.Errorf("Expected no error when stopping without starting, got %v", err)
	}
}
