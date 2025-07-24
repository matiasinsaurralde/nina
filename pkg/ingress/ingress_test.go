package ingress

import (
	"testing"

	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/store"
)

func TestIngressRouting(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{}

	// Create test logger
	log := logger.New(logger.LevelDebug, "text")

	// Create mock store (we don't need Redis for this test)
	var st *store.Store

	// Create ingress
	ing := NewIngress(cfg, log, st)

	// Test cases for routing logic
	testCases := []struct {
		name     string
		host     string
		expected string
	}{
		{
			name:     "Test with Host header",
			host:     "test.example.com",
			expected: "https://httpbin.org",
		},
		{
			name:     "Test with different host",
			host:     "another.example.com",
			expected: "https://httpbin.org",
		},
		{
			name:     "Test with host and port",
			host:     "test.example.com:8080",
			expected: "https://httpbin.org",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the routing logic directly
			url, err := ing.getTargetForHost(tc.host)
			if err != nil {
				t.Fatalf("Failed to get target for host '%s': %v", tc.host, err)
			}

			if url.String() != tc.expected {
				t.Errorf("Expected target '%s', got '%s'", tc.expected, url.String())
			}

			t.Logf("Test passed: %s -> %s", tc.host, url.String())
		})
	}
}

func TestGetTargetForHost(t *testing.T) {
	cfg := &config.Config{}
	log := logger.New(logger.LevelDebug, "text")
	var st *store.Store

	ing := NewIngress(cfg, log, st)

	testCases := []struct {
		name        string
		host        string
		expectError bool
	}{
		{
			name:        "Valid host",
			host:        "test.example.com",
			expectError: false,
		},
		{
			name:        "Empty host",
			host:        "",
			expectError: true,
		},
		{
			name:        "Host with port",
			host:        "test.example.com:8080",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url, err := ing.getTargetForHost(tc.host)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for host '%s', but got none", tc.host)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for host '%s': %v", tc.host, err)
				}
				if url == nil {
					t.Errorf("Expected URL for host '%s', but got nil", tc.host)
				}
				if url.String() != "https://httpbin.org" {
					t.Errorf("Expected target 'https://httpbin.org', got '%s'", url.String())
				}
			}
		})
	}
}
