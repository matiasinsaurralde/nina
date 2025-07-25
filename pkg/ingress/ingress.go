// Package ingress provides reverse proxy and routing functionality for the Nina application.
package ingress

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/store"
	"github.com/matiasinsaurralde/nina/pkg/types"
)

const (
	// DefaultDeploymentRefreshInterval is the default interval for refreshing deployments
	DefaultDeploymentRefreshInterval = 5 * time.Second
)

// Ingress represents the reverse proxy ingress
type Ingress struct {
	config *config.Config
	logger *logger.Logger
	store  *store.Store
	server *http.Server

	// Global deployments state
	deployments     []*types.Deployment
	deploymentsMux  sync.RWMutex
	refreshInterval time.Duration

	// Background goroutine control
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// Route represents a routing rule
type Route struct {
	Host   string `json:"host"`
	Target string `json:"target"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// NewIngress creates a new ingress instance
func NewIngress(cfg *config.Config, log *logger.Logger, st *store.Store) *Ingress {
	refreshInterval := DefaultDeploymentRefreshInterval
	if cfg.Ingress.DeploymentRefreshInterval > 0 {
		refreshInterval = time.Duration(cfg.Ingress.DeploymentRefreshInterval) * time.Second
	}

	return &Ingress{
		config:          cfg,
		logger:          log,
		store:           st,
		refreshInterval: refreshInterval,
		stopChan:        make(chan struct{}),
	}
}

// Start starts the ingress server
func (i *Ingress) Start(ctx context.Context) error {
	// Start the background goroutine for fetching deployments
	i.wg.Add(1)
	go i.deploymentFetcher()

	mux := http.NewServeMux()
	mux.HandleFunc("/", i.handleRequest)

	i.server = &http.Server{
		Addr:              i.config.GetIngressAddr(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	i.logger.Info("Starting ingress server", "addr", i.config.GetIngressAddr(), "refresh_interval", i.refreshInterval)

	go func() {
		if err := i.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			i.logger.Error("Failed to start ingress server", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	return i.Stop(context.Background())
}

// Stop stops the ingress server
func (i *Ingress) Stop(ctx context.Context) error {
	i.logger.Info("Stopping ingress server")

	// Stop the background goroutine
	close(i.stopChan)
	i.wg.Wait()

	if i.server != nil {
		return fmt.Errorf("failed to shutdown ingress: %w", i.server.Shutdown(ctx))
	}
	return nil
}

// deploymentFetcher runs in a background goroutine and fetches deployments periodically
func (i *Ingress) deploymentFetcher() {
	defer i.wg.Done()

	ticker := time.NewTicker(i.refreshInterval)
	defer ticker.Stop()

	// Fetch deployments immediately on startup
	i.fetchDeployments()

	for {
		select {
		case <-ticker.C:
			i.fetchDeployments()
		case <-i.stopChan:
			i.logger.Info("Stopping deployment fetcher")
			return
		}
	}
}

// fetchDeployments fetches deployments from the store and updates the global state
func (i *Ingress) fetchDeployments() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	deployments, err := i.store.ListNewDeployments(ctx)
	if err != nil {
		i.logger.Error("Failed to fetch deployments", "error", err)
		return
	}

	i.deploymentsMux.Lock()
	i.deployments = deployments
	i.deploymentsMux.Unlock()

	i.logger.Debug("Updated deployments cache", "count", len(deployments))
}

// getDeployments returns a copy of the current deployments
func (i *Ingress) getDeployments() []*types.Deployment {
	i.deploymentsMux.RLock()
	defer i.deploymentsMux.RUnlock()

	// Return a copy to avoid race conditions
	deployments := make([]*types.Deployment, len(i.deployments))
	copy(deployments, i.deployments)
	return deployments
}

// handleRequest handles incoming HTTP requests
func (i *Ingress) handleRequest(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		host = r.Header.Get("Host")
	}

	// Remove port from host if present
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	i.logger.Debug("Received request", "host", host, "path", r.URL.Path, "method", r.Method)

	// Find deployment by appName (host)
	deployment := i.findDeploymentByAppName(host)
	if deployment == nil {
		i.logger.Warn("Unknown application", "host", host)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		errorResp := ErrorResponse{
			Error:   "unknown_application",
			Message: "unknown application",
		}

		if err := json.NewEncoder(w).Encode(errorResp); err != nil {
			i.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	// Select a random replica
	container := i.selectRandomReplica(deployment)
	if container == nil {
		i.logger.Error("No available replicas", "app_name", deployment.AppName)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)

		errorResp := ErrorResponse{
			Error:   "no_replicas_available",
			Message: "no replicas available",
		}

		if err := json.NewEncoder(w).Encode(errorResp); err != nil {
			i.logger.Error("Failed to encode error response", "error", err)
		}
		return
	}

	// Build target URL
	targetURL := fmt.Sprintf("http://%s:%d", container.Address, container.Port)
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		i.logger.Error("Failed to parse target URL", "target", targetURL, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	i.logger.Info("Routing request",
		"host", host,
		"app_name", deployment.AppName,
		"target", targetURL,
		"container_id", container.ContainerID)

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(parsedURL)

	// Add custom director to modify request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = parsedURL.Host
		// Inject the container ID header
		req.Header.Set("X-Nina-Replica-Container-ID", container.ContainerID)
	}

	// Add custom transport for better error handling
	proxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Add error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		i.logger.Error("Proxy error", "host", host, "target", targetURL, "error", err)
		http.Error(w, "Proxy error", http.StatusBadGateway)
	}

	// Serve the request
	proxy.ServeHTTP(w, r)
}

// findDeploymentByAppName finds a deployment by appName
func (i *Ingress) findDeploymentByAppName(appName string) *types.Deployment {
	deployments := i.getDeployments()

	for _, deployment := range deployments {
		if deployment.AppName == appName {
			return deployment
		}
	}

	return nil
}

// selectRandomReplica selects a random replica from the deployment's containers
func (i *Ingress) selectRandomReplica(deployment *types.Deployment) *types.Container {
	if len(deployment.Containers) == 0 {
		return nil
	}

	// Use a simple random selection
	randomIndex := rand.Intn(len(deployment.Containers))
	return &deployment.Containers[randomIndex]
}

// getTargetForHost returns the target URL for a given host
func (i *Ingress) getTargetForHost(host string) (*url.URL, error) {
	// For now, route all requests to httpbin.org
	// In a real implementation, this would look up the host in the store
	// and return the appropriate container URL

	if host == "" {
		return nil, fmt.Errorf("empty host")
	}

	// Default routing to httpbin.org for demonstration
	target := "https://httpbin.org"

	// In a real implementation, you would:
	// 1. Look up the host in Redis/store
	// 2. Find the corresponding deployment
	// 3. Return the container's URL

	i.logger.Debug("Routing host to target", "host", host, "target", target)

	parsedURL, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target URL: %w", err)
	}
	return parsedURL, nil
}

// AddRoute adds a new routing rule
func (i *Ingress) AddRoute(host, target string) error {
	// In a real implementation, this would store the route in Redis
	i.logger.Info("Adding route", "host", host, "target", target)
	return nil
}

// RemoveRoute removes a routing rule
func (i *Ingress) RemoveRoute(host string) error {
	// In a real implementation, this would remove the route from Redis
	i.logger.Info("Removing route", "host", host)
	return nil
}
