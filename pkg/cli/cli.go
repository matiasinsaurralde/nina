// Package cli provides command-line interface functionality for the Nina application.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/matiasinsaurralde/nina/internal/pkg/archive"
	"github.com/matiasinsaurralde/nina/internal/pkg/git"
	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/store"
	"github.com/matiasinsaurralde/nina/pkg/types"
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
			Timeout: 5 * time.Minute,
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
	defer resp.Body.Close() //nolint:errcheck

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

// Deploy deploys an application from the current directory
func (c *CLI) Deploy(ctx context.Context, workingDir string, replicas int) (*types.Deployment, error) {
	// Check if the working directory is a Git repository
	if !git.IsGitRepository(workingDir) {
		return nil, fmt.Errorf("directory is not a Git repository: %s", workingDir)
	}

	// Get repository URL
	repoURL, err := git.GetRepoURL(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository URL: %w", err)
	}

	// Extract app name from repository URL
	appName, err := git.ExtractAppNameFromRepoURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract app name from repository URL: %w", err)
	}

	// Get last commit information
	commitInfo, err := git.GetLastCommitInfo(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get last commit information: %w", err)
	}

	// Check if deployment already exists for this app
	exists, err := c.DeploymentExists(ctx, appName)
	if err != nil {
		return nil, fmt.Errorf("failed to check if deployment exists: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("a deployment for app %s already exists", appName)
	}

	// Create deployment request
	req := &types.DeploymentRequest{
		AppName:       appName,
		CommitHash:    commitInfo.Hash,
		Author:        commitInfo.Author,
		AuthorEmail:   commitInfo.Email,
		CommitMessage: commitInfo.Message,
		Replicas:      replicas,
	}

	// Send request to API
	url := fmt.Sprintf("http://%s/api/v1/deploy", c.config.GetServerAddr())

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
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("deploy failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	var deployment types.Deployment
	if err := json.Unmarshal(body, &deployment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &deployment, nil
}

// DeleteDeployment deletes a deployment
func (c *CLI) DeleteDeployment(ctx context.Context, id string) error {
	url := fmt.Sprintf("http://%s/api/v1/deployments/%s", c.config.GetServerAddr(), id)

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	return nil
}

// GetDeploymentStatus gets the status of a deployment
func (c *CLI) GetDeploymentStatus(ctx context.Context, id string) (*store.Deployment, error) {
	url := fmt.Sprintf("http://%s/api/v1/deployments/%s/status", c.config.GetServerAddr(), id)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

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
func (c *CLI) ListDeployments(ctx context.Context) ([]*types.Deployment, error) {
	url := fmt.Sprintf("http://%s/api/v1/deployments", c.config.GetServerAddr())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list deployments failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	var response struct {
		Deployments []*types.Deployment `json:"deployments"`
		Count       int                 `json:"count"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Deployments, nil
}

// HealthCheck checks if the Engine server is healthy
func (c *CLI) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/health", c.config.GetServerAddr())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	return nil
}

// Build builds a deployment from the current directory
func (c *CLI) Build(ctx context.Context, workingDir string) (*types.DeploymentImage, error) {
	// Check if the working directory is a Git repository
	if !git.IsGitRepository(workingDir) {
		return nil, fmt.Errorf("directory is not a Git repository: %s", workingDir)
	}

	// Get repository URL
	repoURL, err := git.GetRepoURL(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository URL: %w", err)
	}

	// Extract app name from repository URL
	appName, err := git.ExtractAppNameFromRepoURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract app name from repository URL: %w", err)
	}

	// Get last commit information
	commitInfo, err := git.GetLastCommitInfo(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get last commit information: %w", err)
	}

	// Check if build already exists for this commit
	exists, err := c.BuildExists(ctx, commitInfo.Hash)
	if err != nil {
		return nil, fmt.Errorf("failed to check if build exists: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("a build for commit %s already exists", commitInfo.Hash)
	}

	// Create temporary directory and copy contents
	tempDir, err := archive.CreateTempDirAndCopy(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create gzipped tar base64
	bundleContents, err := archive.CreateGzippedTarBase64(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzipped tar archive: %w", err)
	}

	// Create build request
	req := &types.BuildRequest{
		AppName:        appName,
		RepoURL:        repoURL,
		Author:         commitInfo.Author,
		AuthorEmail:    commitInfo.Email,
		CommitHash:     commitInfo.Hash,
		CommitMessage:  commitInfo.Message,
		BundleContents: bundleContents,
	}

	// Send request to API
	url := fmt.Sprintf("http://%s/api/v1/build", c.config.GetServerAddr())

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
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("build failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	var deploymentImage types.DeploymentImage
	if err := json.Unmarshal(body, &deploymentImage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &deploymentImage, nil
}

// ListBuilds lists all builds
func (c *CLI) ListBuilds(ctx context.Context) ([]*types.Build, error) {
	url := fmt.Sprintf("http://%s/api/v1/builds", c.config.GetServerAddr())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list builds failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	var response struct {
		Builds []*types.Build `json:"builds"`
		Count  int            `json:"count"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Builds, nil
}

// BuildExists checks if a build exists for the given commit hash
func (c *CLI) BuildExists(ctx context.Context, commitHash string) (bool, error) {
	url := fmt.Sprintf("http://%s/api/v1/builds?commit_hash=%s", c.config.GetServerAddr(), commitHash)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("check build exists failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	var response struct {
		Builds []*types.Build `json:"builds"`
		Count  int            `json:"count"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return false, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return len(response.Builds) > 0, nil
}

// DeploymentExists checks if a deployment exists for the given app name
func (c *CLI) DeploymentExists(ctx context.Context, appName string) (bool, error) {
	url := fmt.Sprintf("http://%s/api/v1/deployments?app_name=%s", c.config.GetServerAddr(), appName)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("check deployment exists failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	var response struct {
		Deployments []*types.Deployment `json:"deployments"`
		Count       int                 `json:"count"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return false, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return len(response.Deployments) > 0, nil
}

func (c *CLI) Config() *config.Config { return c.config }
func (c *CLI) Client() *http.Client   { return c.client }
