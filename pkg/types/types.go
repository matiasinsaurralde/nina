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
	// DeploymentStatusFailed represents a deployment that failed.
	DeploymentStatusFailed DeploymentStatus = "failed"

	// BuildStatusPending represents a build that is pending.
	BuildStatusPending BuildStatus = "pending"
	// BuildStatusBuilding represents a build that is building.
	BuildStatusBuilding BuildStatus = "building"
	// BuildStatusBuilt represents a build that is built.
	BuildStatusBuilt BuildStatus = "built"
	// BuildStatusFailed represents a build that failed.
	BuildStatusFailed BuildStatus = "failed"
)

// DeploymentRequest represents a request to deploy an application.
type DeploymentRequest struct {
	AppName       string `json:"app_name"`
	CommitHash    string `json:"commit_hash"`
	Author        string `json:"author"`
	AuthorEmail   string `json:"author_email"`
	CommitMessage string `json:"commit_message"`
}

// Deployment represents a deployment configuration.
type Deployment struct {
	ID            string           `json:"id"`
	AppName       string           `json:"app_name"`
	RepoURL       string           `json:"repo_url"`
	Author        string           `json:"author"`
	AuthorEmail   string           `json:"author_email"`
	CommitHash    string           `json:"commit_hash"`
	CommitMessage string           `json:"commit_message"`
	Containers    []Container      `json:"containers"`
	Status        DeploymentStatus `json:"status"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
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
	Address     string `json:"address"`
	Port        int    `json:"port"`
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
	Size          int64       `json:"size"`
	Status        BuildStatus `json:"status"`
}
