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
	"reflect"
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

// Provision provisions a new deployment
func (c *CLI) Provision(ctx context.Context, req *store.ProvisionRequest) (*store.Deployment, error) {
	body, err := c.makeJSONRequest(ctx, "provision", req, "provision")
	if err != nil {
		return nil, err
	}

	var deployment store.Deployment
	if err := json.Unmarshal(body, &deployment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &deployment, nil
}

// validateGitRepository validates that the working directory is a Git repository
func (c *CLI) validateGitRepository(workingDir string) error {
	if !git.IsGitRepository(workingDir) {
		return fmt.Errorf("directory is not a Git repository: %s", workingDir)
	}
	return nil
}

// getRepositoryInfo gets repository information from the working directory
func (c *CLI) getRepositoryInfo(workingDir string) (string, *git.CommitInfo, error) {
	// Get repository URL
	repoURL, err := git.GetRepoURL(workingDir)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get repository URL: %w", err)
	}

	// Extract app name from repository URL
	appName, err := git.ExtractAppNameFromRepoURL(repoURL)
	if err != nil {
		return "", nil, fmt.Errorf("failed to extract app name from repository URL: %w", err)
	}

	// Get last commit information
	commitInfo, err := git.GetLastCommitInfo(workingDir)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get last commit information: %w", err)
	}

	return appName, commitInfo, nil
}

// createDeploymentRequest creates a deployment request from repository info
func (c *CLI) createDeploymentRequest(appName string, commitInfo *git.CommitInfo, replicas int) *types.DeploymentRequest {
	return &types.DeploymentRequest{
		AppName:       appName,
		CommitHash:    commitInfo.Hash,
		Author:        commitInfo.Author,
		AuthorEmail:   commitInfo.Email,
		CommitMessage: commitInfo.Message,
		Replicas:      replicas,
	}
}

// sendDeploymentRequest sends the deployment request to the API
func (c *CLI) sendDeploymentRequest(ctx context.Context, req *types.DeploymentRequest) (*types.Deployment, error) {
	body, err := c.makeJSONRequest(ctx, "deploy", req, "deploy")
	if err != nil {
		return nil, err
	}

	var deployment types.Deployment
	if err := json.Unmarshal(body, &deployment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &deployment, nil
}

// Deploy deploys an application from the current directory
func (c *CLI) Deploy(ctx context.Context, workingDir string, replicas int) (*types.Deployment, error) {
	// Validate Git repository
	if err := c.validateGitRepository(workingDir); err != nil {
		return nil, err
	}

	// Get repository information
	appName, commitInfo, err := c.getRepositoryInfo(workingDir)
	if err != nil {
		return nil, err
	}

	// Check if deployment already exists for this app
	exists, err := c.DeploymentExists(ctx, appName)
	if err != nil {
		return nil, fmt.Errorf("failed to check if deployment exists: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("a deployment for app %s already exists", appName)
	}

	// Create and send deployment request
	req := c.createDeploymentRequest(appName, commitInfo, replicas)
	return c.sendDeploymentRequest(ctx, req)
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
	body, err := c.makeListRequest(ctx, "deployments", "deployments")
	if err != nil {
		return nil, err
	}

	response, err := unmarshalListResponse(body, "deployments")
	if err != nil {
		return nil, err
	}

	return response.([]*types.Deployment), nil
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

// createBuildBundle creates a build bundle from the working directory
func (c *CLI) createBuildBundle(workingDir string) (string, error) {
	// Create temporary directory and copy contents
	tempDir, err := archive.CreateTempDirAndCopy(workingDir)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			c.logger.Error("Failed to remove temp directory", "error", removeErr)
		}
	}()

	// Create gzipped tar base64
	bundleContents, err := archive.CreateGzippedTarBase64(tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to create gzipped tar archive: %w", err)
	}

	return bundleContents, nil
}

// createBuildRequest creates a build request from repository info and bundle contents
func (c *CLI) createBuildRequest(appName, repoURL, bundleContents string, commitInfo *git.CommitInfo) *types.BuildRequest {
	return &types.BuildRequest{
		AppName:        appName,
		RepoURL:        repoURL,
		Author:         commitInfo.Author,
		AuthorEmail:    commitInfo.Email,
		CommitHash:     commitInfo.Hash,
		CommitMessage:  commitInfo.Message,
		BundleContents: bundleContents,
	}
}

// sendBuildRequest sends the build request to the API
func (c *CLI) sendBuildRequest(ctx context.Context, req *types.BuildRequest) (*types.DeploymentImage, error) {
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

// Build builds a deployment from the current directory
func (c *CLI) Build(ctx context.Context, workingDir string) (*types.DeploymentImage, error) {
	// Validate Git repository
	if err := c.validateGitRepository(workingDir); err != nil {
		return nil, err
	}

	// Get repository information
	appName, commitInfo, err := c.getRepositoryInfo(workingDir)
	if err != nil {
		return nil, err
	}

	// Get repository URL
	repoURL, err := git.GetRepoURL(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository URL: %w", err)
	}

	// Check if build already exists for this commit
	exists, err := c.BuildExists(ctx, commitInfo.Hash)
	if err != nil {
		return nil, fmt.Errorf("failed to check if build exists: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("a build for commit %s already exists", commitInfo.Hash)
	}

	// Create build bundle
	bundleContents, err := c.createBuildBundle(workingDir)
	if err != nil {
		return nil, err
	}

	// Create and send build request
	req := c.createBuildRequest(appName, repoURL, bundleContents, commitInfo)
	return c.sendBuildRequest(ctx, req)
}

// ListBuilds lists all builds
func (c *CLI) ListBuilds(ctx context.Context) ([]*types.Build, error) {
	body, err := c.makeListRequest(ctx, "builds", "builds")
	if err != nil {
		return nil, err
	}

	response, err := unmarshalListResponse(body, "builds")
	if err != nil {
		return nil, err
	}

	return response.([]*types.Build), nil
}

// BuildExists checks if a build exists for the given commit hash
func (c *CLI) BuildExists(ctx context.Context, commitHash string) (bool, error) {
	return c.makeExistsRequest(ctx, "builds", "commit_hash", commitHash, "builds")
}

// DeploymentExists checks if a deployment exists for the given app name
func (c *CLI) DeploymentExists(ctx context.Context, appName string) (bool, error) {
	return c.makeExistsRequest(ctx, "deployments", "app_name", appName, "deployments")
}

// Config returns the CLI configuration.
func (c *CLI) Config() *config.Config { return c.config }

// Client returns the HTTP client.
func (c *CLI) Client() *http.Client { return c.client }

// makeHTTPRequest is a helper function to make HTTP requests and handle common response processing
func (c *CLI) makeHTTPRequest(ctx context.Context, url string) ([]byte, error) {
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
		return nil, fmt.Errorf("request failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	return body, nil
}

// makeListRequest is a helper function to make list requests
func (c *CLI) makeListRequest(ctx context.Context, endpoint, responseType string) ([]byte, error) {
	url := fmt.Sprintf("http://%s/api/v1/%s", c.config.GetServerAddr(), endpoint)

	body, err := c.makeHTTPRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("list %s failed: %w", responseType, err)
	}

	return body, nil
}

// unmarshalListResponse is a helper function to unmarshal list responses
func unmarshalListResponse(body []byte, responseType string) (interface{}, error) {
	var response interface{}

	switch responseType {
	case "deployments":
		var resp struct {
			Deployments []*types.Deployment `json:"deployments"`
			Count       int                 `json:"count"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		response = resp.Deployments
	case "builds":
		var resp struct {
			Builds []*types.Build `json:"builds"`
			Count  int            `json:"count"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		response = resp.Builds
	default:
		return nil, fmt.Errorf("unknown response type: %s", responseType)
	}

	return response, nil
}

// makeExistsRequest is a helper function to make exists requests
func (c *CLI) makeExistsRequest(ctx context.Context, endpoint, param, value, responseType string) (bool, error) {
	url := fmt.Sprintf("http://%s/api/v1/%s?%s=%s", c.config.GetServerAddr(), endpoint, param, value)

	body, err := c.makeHTTPRequest(ctx, url)
	if err != nil {
		return false, fmt.Errorf("check %s exists failed: %w", responseType, err)
	}

	response, err := unmarshalListResponse(body, responseType)
	if err != nil {
		return false, err
	}

	// Use reflection to get the length of the slice
	responseValue := reflect.ValueOf(response)
	if responseValue.Kind() == reflect.Slice {
		return responseValue.Len() > 0, nil
	}

	return false, nil
}

// makeJSONRequest is a generic helper for making JSON HTTP requests
func (c *CLI) makeJSONRequest(ctx context.Context, endpoint string, req interface{}, responseType string) ([]byte, error) {
	url := fmt.Sprintf("http://%s/api/v1/%s", c.config.GetServerAddr(), endpoint)

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
		return nil, fmt.Errorf("%s failed: %s (status: %d)", responseType, string(body), resp.StatusCode)
	}

	return body, nil
}
