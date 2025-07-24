// Package store provides data storage functionality for the Nina application.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/matiasinsaurralde/nina/pkg/config"
	"github.com/matiasinsaurralde/nina/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// Store represents the Redis store
type Store struct {
	client *redis.Client
	logger *logger.Logger
	config *config.Config
}

// Deployment represents a container deployment
type Deployment struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Status      string            `json:"status"`
	Ports       []int             `json:"ports"`
	Environment map[string]string `json:"environment"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ProvisionRequest represents a request to provision a container
type ProvisionRequest struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Ports       []int             `json:"ports"`
	Environment map[string]string `json:"environment"`
}

// NewStore creates a new Redis store instance
func NewStore(cfg *config.Config, log *logger.Logger) (*Store, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Info("Connected to Redis", "addr", cfg.GetRedisAddr())

	return &Store{
		client: client,
		logger: log,
		config: cfg,
	}, nil
}

// Close closes the Redis connection
func (s *Store) Close() error {
	if err := s.client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis client: %w", err)
	}
	return nil
}

// CreateDeployment creates a new deployment
func (s *Store) CreateDeployment(ctx context.Context, req *ProvisionRequest) (*Deployment, error) {
	deployment := &Deployment{
		ID:          generateID(),
		Name:        req.Name,
		Image:       req.Image,
		Status:      "creating",
		Ports:       req.Ports,
		Environment: req.Environment,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Store deployment data
	key := fmt.Sprintf("deployment:%s", deployment.ID)
	data, err := json.Marshal(deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment: %w", err)
	}

	if err := s.client.Set(ctx, key, data, 0).Err(); err != nil {
		return nil, fmt.Errorf("failed to store deployment: %w", err)
	}

	// Store deployment ID by name for quick lookup
	nameKey := fmt.Sprintf("deployment:name:%s", deployment.Name)
	if err := s.client.Set(ctx, nameKey, deployment.ID, 0).Err(); err != nil {
		return nil, fmt.Errorf("failed to store deployment name mapping: %w", err)
	}

	s.logger.Info("Created deployment", "id", deployment.ID, "name", deployment.Name)
	return deployment, nil
}

// GetDeployment retrieves a deployment by ID
func (s *Store) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	key := fmt.Sprintf("deployment:%s", id)
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("deployment not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	var deployment Deployment
	if err := json.Unmarshal(data, &deployment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment: %w", err)
	}

	return &deployment, nil
}

// GetDeploymentByName retrieves a deployment by name
func (s *Store) GetDeploymentByName(ctx context.Context, name string) (*Deployment, error) {
	nameKey := fmt.Sprintf("deployment:name:%s", name)
	id, err := s.client.Get(ctx, nameKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("deployment not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get deployment ID by name: %w", err)
	}

	return s.GetDeployment(ctx, id)
}

// UpdateDeploymentStatus updates the status of a deployment
func (s *Store) UpdateDeploymentStatus(ctx context.Context, id, status string) error {
	deployment, err := s.GetDeployment(ctx, id)
	if err != nil {
		return err
	}

	deployment.Status = status
	deployment.UpdatedAt = time.Now()

	key := fmt.Sprintf("deployment:%s", id)
	data, err := json.Marshal(deployment)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment: %w", err)
	}

	if err := s.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	s.logger.Info("Updated deployment status", "id", id, "status", status)
	return nil
}

// DeleteDeployment deletes a deployment
func (s *Store) DeleteDeployment(ctx context.Context, id string) error {
	deployment, err := s.GetDeployment(ctx, id)
	if err != nil {
		return err
	}

	// Delete deployment data
	key := fmt.Sprintf("deployment:%s", id)
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	// Delete name mapping
	nameKey := fmt.Sprintf("deployment:name:%s", deployment.Name)
	if err := s.client.Del(ctx, nameKey).Err(); err != nil {
		return fmt.Errorf("failed to delete deployment name mapping: %w", err)
	}

	s.logger.Info("Deleted deployment", "id", id, "name", deployment.Name)
	return nil
}

// ListDeployments retrieves all deployments
func (s *Store) ListDeployments(ctx context.Context) ([]*Deployment, error) {
	pattern := "deployment:*"
	keys, err := s.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment keys: %w", err)
	}

	deployments := make([]*Deployment, 0, len(keys))
	for _, key := range keys {
		// Skip name mappings
		if len(key) > 14 && key[:14] == "deployment:name" {
			continue
		}

		// Only process actual deployment keys (not name mappings)
		if strings.HasPrefix(key, "deployment:name:") {
			continue
		}

		data, err := s.client.Get(ctx, key).Bytes()
		if err != nil {
			s.logger.Warn("Failed to get deployment data", "key", key, "error", err)
			continue
		}

		var deployment Deployment
		if err := json.Unmarshal(data, &deployment); err != nil {
			s.logger.Warn("Failed to unmarshal deployment", "key", key, "error", err)
			continue
		}

		deployments = append(deployments, &deployment)
	}

	return deployments, nil
}

// generateID generates a simple ID for deployments
func generateID() string {
	return fmt.Sprintf("deploy-%d", time.Now().UnixNano())
}
