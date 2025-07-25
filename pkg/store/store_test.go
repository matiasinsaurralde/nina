package store

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
)

func TestStoreWithMiniredis(t *testing.T) {
	// Start Miniredis
	mockRedis, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start Miniredis: %v", err)
	}
	defer mockRedis.Close()

	// Create test configuration
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     mockRedis.Host(),
			Port:     mockRedis.Server().Addr().Port,
			Password: "",
			DB:       0,
		},
	}

	// Create test logger
	log := logger.New(logger.LevelDebug, "text")

	// Create store using NewStore function to ensure proper initialization
	store, err := NewStore(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Run the same test suite as integration tests but with mock store
	runStoreTestSuite(t, store)
}
