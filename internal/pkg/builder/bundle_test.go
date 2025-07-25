package builder

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"testing"

	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/types"
)

func TestNewBundleWithLogging(t *testing.T) { //nolint: funlen
	// Create a test logger
	log := logger.New(logger.LevelDebug, "text")

	// Create a simple test tar.gz file
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Add a test file
	content := []byte("test content")
	header := &tar.Header{
		Name: "test.txt",
		Mode: 0o644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("Failed to write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("Failed to write tar content: %v", err)
	}

	// Add a test directory
	header = &tar.Header{
		Name:     "testdir/",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("Failed to write tar header: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Failed to close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}

	// Encode as base64
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Create test request
	req := &types.BuildRequest{
		AppName:        "test-app",
		RepoURL:        "https://github.com/test/test-app",
		Author:         "Test User",
		AuthorEmail:    "test@example.com",
		CommitHash:     "abc123",
		BundleContents: encoded,
	}

	// Test bundle extraction
	bundle, err := NewBundle(req, log)
	if err != nil {
		t.Fatalf("Failed to create bundle: %v", err)
	}
	defer func() {
		if err := bundle.Cleanup(); err != nil {
			t.Logf("Failed to cleanup bundle: %v", err)
		}
	}()

	// Verify bundle was created
	if bundle.GetTempDir() == "" {
		t.Error("Bundle temp directory should not be empty")
	}

	if bundle.GetRequest() != req {
		t.Error("Bundle request should match the original request")
	}

	// Test cleanup
	if err := bundle.Cleanup(); err != nil {
		t.Errorf("Failed to cleanup bundle: %v", err)
	}
}
