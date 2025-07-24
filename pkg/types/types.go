// Package types provides common data structures for the Nina application.
package types

// Deployment represents a deployment configuration.
type Deployment struct {
	AppName string `json:"app_name"`
	RepoURL string `json:"repo_url"`
}

// DeploymentBuildRequest represents a request to build a deployment.
type DeploymentBuildRequest struct {
	AppName       string `json:"app_name"`
	RepoURL       string `json:"repo_url"`
	Author        string `json:"author"`
	AuthorEmail   string `json:"author_email"`
	CommitHash    string `json:"commit_hash"`
	NoContainers  int64  `json:"no_containers"`
	Status        string `json:"status"`
	BundleContent string `json:"bundle_content"`
}

// Container represents a container configuration.
type Container struct {
	ContainerID string `json:"container_id"`
	ImageTag    string `json:"image_tag"`
}
