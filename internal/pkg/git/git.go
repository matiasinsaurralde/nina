// Package git provides functionality for extracting Git repository information.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// CommitInfo represents Git commit information
type CommitInfo struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// GetRepoURL gets the repository URL from the current Git repository
func GetRepoURL(repoPath string) (string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get repository URL: %w", err)
	}

	url := strings.TrimSpace(string(output))
	if url == "" {
		return "", fmt.Errorf("no remote origin URL found")
	}

	return url, nil
}

// ExtractAppNameFromRepoURL extracts the application name from a repository URL
func ExtractAppNameFromRepoURL(repoURL string) (string, error) {
	if repoURL == "" {
		return "", fmt.Errorf("repository URL is empty")
	}

	// Split by "/" and get the last part
	parts := strings.Split(repoURL, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid repository URL format")
	}

	lastPart := parts[len(parts)-1]

	// Remove ".git" suffix if present
	appName := strings.TrimSuffix(lastPart, ".git")

	if appName == "" {
		return "", fmt.Errorf("could not extract app name from repository URL")
	}

	return appName, nil
}

// GetLastCommitInfo gets information about the last commit in the repository
func GetLastCommitInfo(repoPath string) (*CommitInfo, error) {
	// Get commit hash
	hashCmd := exec.Command("git", "rev-parse", "HEAD")
	hashCmd.Dir = repoPath
	hashOutput, err := hashCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit hash: %w", err)
	}
	hash := strings.TrimSpace(string(hashOutput))

	// Get author name
	authorCmd := exec.Command("git", "log", "-1", "--pretty=format:%an")
	authorCmd.Dir = repoPath
	authorOutput, err := authorCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get author name: %w", err)
	}
	author := strings.TrimSpace(string(authorOutput))

	// Get author email
	emailCmd := exec.Command("git", "log", "-1", "--pretty=format:%ae")
	emailCmd.Dir = repoPath
	emailOutput, err := emailCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get author email: %w", err)
	}
	email := strings.TrimSpace(string(emailOutput))

	// Get commit message
	messageCmd := exec.Command("git", "log", "-1", "--pretty=format:%s")
	messageCmd.Dir = repoPath
	messageOutput, err := messageCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit message: %w", err)
	}
	message := strings.TrimSpace(string(messageOutput))

	return &CommitInfo{
		Hash:    hash,
		Author:  author,
		Email:   email,
		Message: message,
	}, nil
}

// IsGitRepository checks if the given path is a Git repository
func IsGitRepository(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path

	err := cmd.Run()
	return err == nil
}
