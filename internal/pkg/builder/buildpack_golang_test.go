package builder

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/matiasinsaurralde/nina/pkg/types"
	"github.com/stretchr/testify/assert"
)

// loadTestBundle loads the test data file and returns it as base64 encoded string
func loadTestBundle(t *testing.T) string {
	t.Helper()

	// Read the test data file from the project root
	data, err := os.ReadFile(
		filepath.Join(
			"..",
			"..",
			"..",
			"testdata",
			"nina-test-app.tar.gz",
		))
	assert.NoError(t, err)

	// Encode as base64
	encoded := base64.StdEncoding.EncodeToString(data)
	return encoded
}

func TestBuildpackGolang_Match(t *testing.T) {
	buildpack := &BuildpackGolang{
		BaseBuildpack: &BaseBuildpack{},
	}

	// Create a logger for the test
	log := logger.New(logger.LevelDebug, "text")

	// Load and encode the test bundle
	bundleContents := loadTestBundle(t)

	bundle, err := NewBundle(&types.BuildRequest{
		BundleContents: bundleContents,
	}, log)
	assert.NoError(t, err)

	match, err := buildpack.Match(context.Background(), bundle)
	assert.NoError(t, err)
	assert.True(t, match)
}
