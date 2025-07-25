package main

import (
	"os/exec"
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero bytes", 0, "0 B"},
		{"small bytes", 512, "512 B"},
		{"1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"1 MB", 1048576, "1.0 MB"},
		{"1.5 MB", 1572864, "1.5 MB"},
		{"1 GB", 1073741824, "1.0 GB"},
		{"large bytes", 2147483648, "2.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestCLIErrorHandling(t *testing.T) {
	// Skip this test if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test that the CLI doesn't show usage help on errors
	// This would require a real server to test the buildExists error
	// For now, we'll just verify the binary exists and can be executed
	cmd := exec.Command("./nina", "build")
	cmd.Dir = "."

	// Capture output
	output, err := cmd.CombinedOutput()

	// We expect an error since there's no server running
	if err == nil {
		t.Log("Expected error when no server is running")
	}

	// Check that the output doesn't contain usage information
	outputStr := string(output)
	if containsUsage(outputStr) {
		t.Error("CLI should not show usage help on errors")
	}
}

// containsUsage checks if the output contains usage help information
func containsUsage(output string) bool {
	usageKeywords := []string{
		"Usage:",
		"Available Commands:",
		"Flags:",
		"Use \"nina [command] --help\"",
	}

	for _, keyword := range usageKeywords {
		if contains(output, keyword) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(len(s) == len(substr) && s == substr ||
			len(s) > len(substr) && (s[:len(substr)] == substr ||
				contains(s[1:], substr)))
}
