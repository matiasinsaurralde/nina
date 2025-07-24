//go:build integration
// +build integration

package store

import (
	"context"
	"fmt"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// MockStore represents a store that can use either real Redis or Miniredis
type MockStore struct {
	*Store
	mockRedis *miniredis.Miniredis
}

// NewMockStore creates a new mock store instance
func NewMockStore(cfg *config.Config, log *logger.Logger) (*MockStore, error) {
	// Try to connect to real Redis first
	realClient := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := realClient.Ping(ctx).Err(); err == nil {
		// Real Redis is available, use it
		log.Info("Using real Redis for integration tests")
		store, err := NewStore(cfg, log)
		if err != nil {
			return nil, err
		}
		return &MockStore{Store: store}, nil
	}

	// Real Redis not available, use Miniredis
	log.Info("Real Redis not available, using Miniredis for integration tests")

	mockRedis, err := miniredis.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to start Miniredis: %w", err)
	}

	mockClient := redis.NewClient(&redis.Options{
		Addr: mockRedis.Addr(),
	})

	// Test mock connection
	if err := mockClient.Ping(ctx).Err(); err != nil {
		mockRedis.Close()
		return nil, fmt.Errorf("failed to connect to Miniredis: %w", err)
	}

	store := &Store{
		client: mockClient,
		logger: log,
		config: cfg,
	}

	return &MockStore{
		Store:     store,
		mockRedis: mockRedis,
	}, nil
}

// Close closes the mock store
func (m *MockStore) Close() error {
	if m.mockRedis != nil {
		m.mockRedis.Close()
	}
	return m.Store.Close()
}

// FlushAll clears all data from the store
func (m *MockStore) FlushAll(ctx context.Context) error {
	return fmt.Errorf("failed to flush all: %w", m.client.FlushAll(ctx).Err())
}
