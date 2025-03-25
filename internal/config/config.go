package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	// CacheTTL is the time-to-live for cached data
	CacheTTL time.Duration
	// DefaultOrg is the default organization to use
	DefaultOrg string
	// Verbose enables verbose logging
	Verbose bool
}

// LoadConfig loads the configuration from the default location
func LoadConfig() (*Config, error) {
	// Set default configuration values
	viper.SetDefault("cache_ttl", "24h")
	viper.SetDefault("default_org", "")
	viper.SetDefault("verbose", false)

	// Set configuration file name and location
	viper.SetConfigName("config")
	viper.SetConfigType("json")

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Add the config directory to the search path
	configDir := filepath.Join(homeDir, ".config", "snyk-auto-org")
	viper.AddConfigPath(configDir)

	// Create the config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read the configuration file
	if err := viper.ReadInConfig(); err != nil {
		// It's okay if the config file doesn't exist
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		// Create a default config file if it doesn't exist
		configPath := filepath.Join(configDir, "config.json")
		if err := viper.WriteConfigAs(configPath); err != nil {
			return nil, fmt.Errorf("failed to create default config file: %w", err)
		}
	}

	// Parse the cache TTL
	cacheTTL, err := time.ParseDuration(viper.GetString("cache_ttl"))
	if err != nil {
		return nil, fmt.Errorf("invalid cache TTL: %w", err)
	}

	// Create and return the config
	return &Config{
		CacheTTL:   cacheTTL,
		DefaultOrg: viper.GetString("default_org"),
		Verbose:    viper.GetBool("verbose"),
	}, nil
}

// SaveConfig saves the configuration to disk
func SaveConfig(cfg *Config) error {
	viper.Set("cache_ttl", cfg.CacheTTL.String())
	viper.Set("default_org", cfg.DefaultOrg)
	viper.Set("verbose", cfg.Verbose)

	return viper.WriteConfig()
}
