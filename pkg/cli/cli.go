package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/store"
)

// CLI represents the command line interface
type CLI struct {
	config *config.Config
	logger *logger.Logger
	client *http.Client
}

// NewCLI creates a new CLI instance
func NewCLI(cfg *config.Config, log *logger.Logger) *CLI {
	return &CLI{
		config: cfg,
		logger: log,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Provision creates a new deployment
func (c *CLI) Provision(ctx context.Context, req *store.ProvisionRequest) (*store.Deployment, error) {
	url := fmt.Sprintf("http://%s/api/v1/provision", c.config.GetServerAddr())

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("provision failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	var deployment store.Deployment
	if err := json.Unmarshal(body, &deployment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &deployment, nil
}

// DeleteDeployment deletes a deployment
func (c *CLI) DeleteDeployment(ctx context.Context, id string) error {
	url := fmt.Sprintf("http://%s/api/v1/deployments/%s", c.config.GetServerAddr(), id)

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	return nil
}

// GetDeploymentStatus gets the status of a deployment
func (c *CLI) GetDeploymentStatus(ctx context.Context, id string) (*store.Deployment, error) {
	url := fmt.Sprintf("http://%s/api/v1/deployments/%s/status", c.config.GetServerAddr(), id)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get status failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	var deployment store.Deployment
	if err := json.Unmarshal(body, &deployment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &deployment, nil
}

// ListDeployments lists all deployments
func (c *CLI) ListDeployments(ctx context.Context) ([]*store.Deployment, error) {
	url := fmt.Sprintf("http://%s/api/v1/deployments", c.config.GetServerAddr())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list deployments failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	var response struct {
		Deployments []*store.Deployment `json:"deployments"`
		Count       int                 `json:"count"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Deployments, nil
}

// HealthCheck checks if the API server is healthy
func (c *CLI) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/health", c.config.GetServerAddr())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	return nil
}
