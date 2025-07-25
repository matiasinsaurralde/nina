// Package config provides configuration management for the Nina application.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	Redis   RedisConfig   `mapstructure:"redis"`
	Logging LoggingConfig `mapstructure:"logging"`
	Ingress IngressConfig `mapstructure:"ingress"`
}

// ServerConfig holds the Engine server configuration
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// RedisConfig holds the Redis connection configuration
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// LoggingConfig holds the logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// IngressConfig holds the ingress proxy configuration
type IngressConfig struct {
	Host                      string `mapstructure:"host"`
	Port                      int    `mapstructure:"port"`
	DeploymentRefreshInterval int    `mapstructure:"deployment_refresh_interval"`
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	// Set default values
	setDefaults()

	// If config path is provided, use it
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		// Use XDG config directory
		configDir := getConfigDir()
		viper.SetConfigName("nina")
		viper.SetConfigType("json")
		viper.AddConfigPath(configDir)
	}

	// Read environment variables
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, create default one
			if createErr := createDefaultConfig(); createErr != nil {
				return nil, fmt.Errorf("failed to create default config: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("ingress.host", "0.0.0.0")
	viper.SetDefault("ingress.port", 8081)
	viper.SetDefault("ingress.deployment_refresh_interval", 5)
}

// getConfigDir returns the XDG-compliant config directory
func getConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory
		return "."
	}

	configDir := filepath.Join(homeDir, ".nina")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		// Fallback to current directory
		return "."
	}

	return configDir
}

// createDefaultConfig creates a default configuration file
func createDefaultConfig() error {
	configDir := getConfigDir()
	configPath := filepath.Join(configDir, "nina.json")

	// Set default values
	setDefaults()

	// Write config file
	return fmt.Errorf("failed to write config: %w", viper.WriteConfigAs(configPath))
}

// GetRedisAddr returns the Redis address string
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}

// GetServerAddr returns the server address string
func (c *Config) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetIngressAddr returns the ingress address string
func (c *Config) GetIngressAddr() string {
	return fmt.Sprintf("%s:%d", c.Ingress.Host, c.Ingress.Port)
}
