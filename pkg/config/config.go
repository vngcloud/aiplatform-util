package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds the configuration for the aiplatform-util tool
type Config struct {
	// AWS/S3 credentials
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string

	// S3 bucket and local mount path
	BucketName string
	MountPath  string
}

const (
	configDir = "/etc/config-nv"
)

// getConfigValue attempts to read configuration from file first, then falls back to environment variable
func getConfigValue(key string) string {
	// Try to read from file in /etc/config-nv/
	filePath := fmt.Sprintf("%s/%s", configDir, key)
	if data, err := os.ReadFile(filePath); err == nil {
		// Trim whitespace and newlines from file content
		return strings.TrimSpace(string(data))
	}

	// Fallback to environment variable
	return os.Getenv(key)
}

// Load reads and validates configuration from /etc/config-nv/ files or environment variables
// Priority: 1. Files in /etc/config-nv/ 2. Environment variables
func Load() (*Config, error) {
	mountPath := getConfigValue("MOUNT_PATH")
	if mountPath == "" {
		// Default to ~/test/workspace
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		mountPath = home + "/test/workspace"
	}

	cfg := &Config{
		AccessKeyID:     getConfigValue("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: getConfigValue("AWS_SECRET_ACCESS_KEY"),
		Endpoint:        getConfigValue("AWS_ENDPOINT"),
		BucketName:      getConfigValue("S3_BUCKET"),
		MountPath:       mountPath,
	}

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that all required configuration fields are set
func (c *Config) Validate() error {
	if c.AccessKeyID == "" {
		return fmt.Errorf("AWS_ACCESS_KEY_ID is required (set via /etc/config-nv/AWS_ACCESS_KEY_ID file or environment variable)")
	}
	if c.SecretAccessKey == "" {
		return fmt.Errorf("AWS_SECRET_ACCESS_KEY is required (set via /etc/config-nv/AWS_SECRET_ACCESS_KEY file or environment variable)")
	}
	if c.Endpoint == "" {
		return fmt.Errorf("AWS_ENDPOINT is required (set via /etc/config-nv/AWS_ENDPOINT file or environment variable)")
	}
	// BucketName is optional - we can list buckets if not provided
	// MountPath has a default value, so no need to validate
	return nil
}
