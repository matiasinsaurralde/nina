// Package ingress provides reverse proxy and routing functionality for the Nina application.
package ingress

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/store"
)

// Ingress represents the reverse proxy ingress
type Ingress struct {
	config *config.Config
	logger *logger.Logger
	store  *store.Store
	server *http.Server
}

// Route represents a routing rule
type Route struct {
	Host   string `json:"host"`
	Target string `json:"target"`
}

// NewIngress creates a new ingress instance
func NewIngress(cfg *config.Config, log *logger.Logger, st *store.Store) *Ingress {
	return &Ingress{
		config: cfg,
		logger: log,
		store:  st,
	}
}

// Start starts the ingress server
func (i *Ingress) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", i.handleRequest)

	i.server = &http.Server{
		Addr:              i.config.GetIngressAddr(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	i.logger.Info("Starting ingress server", "addr", i.config.GetIngressAddr())

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
	if i.server != nil {
		i.logger.Info("Stopping ingress server")
		return fmt.Errorf("failed to shutdown ingress: %w", i.server.Shutdown(ctx))
	}
	return nil
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

	// Route based on host header
	targetURL, err := i.getTargetForHost(host)
	if err != nil {
		i.logger.Warn("No route found for host", "host", host)
		http.Error(w, "No route found for host", http.StatusNotFound)
		return
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Add custom director to modify request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
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
		i.logger.Error("Proxy error", "host", host, "target", targetURL.String(), "error", err)
		http.Error(w, "Proxy error", http.StatusBadGateway)
	}

	// Serve the request
	proxy.ServeHTTP(w, r)
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
