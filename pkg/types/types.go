// Package types provides common data structures for the Nina application.
package types

// DeploymentStatus represents the status of a deployment.
type DeploymentStatus string

const (
	// DeploymentStatusUnavailable represents a deployment that is unavailable.
	DeploymentStatusUnavailable DeploymentStatus = "unavailable"
	// DeploymentStatusDeploying represents a deployment that is deploying.
	DeploymentStatusDeploying DeploymentStatus = "deploying"
	// DeploymentStatusReady represents a deployment that is ready.
	DeploymentStatusReady DeploymentStatus = "ready"
)

// Deployment represents a deployment configuration.
type Deployment struct {
	AppName    string           `json:"app_name"`
	RepoURL    string           `json:"repo_url"`
	Containers []Container      `json:"containers"`
	Status     DeploymentStatus `json:"status"`
}

// Container represents a container configuration.
type Container struct {
	ContainerID string `json:"container_id"`
	ImageTag    string `json:"image_tag"`
}

// DeploymentBuildRequest represents a request to build a deployment.
type DeploymentBuildRequest struct {
	AppName        string `json:"app_name"`
	RepoURL        string `json:"repo_url"`
	Author         string `json:"author"`
	AuthorEmail    string `json:"author_email"`
	CommitHash     string `json:"commit_hash"`
	NoContainers   int64  `json:"no_containers"`
	Status         string `json:"status"`
	BundleContents string `json:"bundle_content"`
}
