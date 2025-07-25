// Package types provides common data structures for the Nina application.
package types

import "time"

const (
	// DefaultNoContainers is the default number of containers for a deployment
	DefaultNoContainers = 1
)

// DeploymentStatus represents the status of a deployment.
type (
	DeploymentStatus string
	BuildStatus      string
)

const (
	// DeploymentStatusUnavailable represents a deployment that is unavailable.
	DeploymentStatusUnavailable DeploymentStatus = "unavailable"
	// DeploymentStatusDeploying represents a deployment that is deploying.
	DeploymentStatusDeploying DeploymentStatus = "deploying"
	// DeploymentStatusReady represents a deployment that is ready.
	DeploymentStatusReady DeploymentStatus = "ready"

	// BuildStatusPending represents a build that is pending.
	BuildStatusPending BuildStatus = "pending"
	// BuildStatusBuilding represents a build that is building.
	BuildStatusBuilding BuildStatus = "building"
	// BuildStatusBuilt represents a build that is built.
	BuildStatusBuilt BuildStatus = "built"
	// BuildStatusFailed represents a build that failed.
	BuildStatusFailed BuildStatus = "failed"
)

// Deployment represents a deployment configuration.
type Deployment struct {
	AppName    string           `json:"app_name"`
	RepoURL    string           `json:"repo_url"`
	Containers []Container      `json:"containers"`
	Status     DeploymentStatus `json:"status"`
}

type DeploymentImage struct {
	ImageTag string `json:"image_tag"`
	ImageID  string `json:"image_id"`
	Size     int64  `json:"size"`
}

// Container represents a container configuration.
type Container struct {
	ContainerID string `json:"container_id"`
	ImageTag    string `json:"image_tag"`
}

// BuildRequest represents a request to build a deployment.
type BuildRequest struct {
	AppName        string `json:"app_name"`
	RepoURL        string `json:"repo_url"`
	Author         string `json:"author"`
	AuthorEmail    string `json:"author_email"`
	CommitHash     string `json:"commit_hash"`
	CommitMessage  string `json:"commit_message"`
	NoContainers   int64  `json:"no_containers"`
	BundleContents string `json:"bundle_content"`
}

type Build struct {
	CreatedAt     time.Time   `json:"created_at"`
	FinishedAt    time.Time   `json:"finished_at"`
	AppName       string      `json:"app_name"`
	RepoURL       string      `json:"repo_url"`
	Author        string      `json:"author"`
	AuthorEmail   string      `json:"author_email"`
	CommitHash    string      `json:"commit_hash"`
	CommitMessage string      `json:"commit_message"`
	ImageTag      string      `json:"image_tag"`
	ImageID       string      `json:"image_id"`
	Status        BuildStatus `json:"status"`
	Size          int64       `json:"size"`
}
