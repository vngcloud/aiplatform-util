package config

import (
	"fmt"
	"os"
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

// Load reads and validates configuration from environment variables
func Load() (*Config, error) {
	mountPath := os.Getenv("MOUNT_PATH")
	if mountPath == "" {
		// Default to ~/test/workspace
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		mountPath = home + "/test/workspace"
	}

	cfg := &Config{
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		Endpoint:        os.Getenv("AWS_ENDPOINT"),
		BucketName:      os.Getenv("S3_BUCKET"),
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
		return fmt.Errorf("AWS_ACCESS_KEY_ID environment variable is required")
	}
	if c.SecretAccessKey == "" {
		return fmt.Errorf("AWS_SECRET_ACCESS_KEY environment variable is required")
	}
	if c.Endpoint == "" {
		return fmt.Errorf("AWS_ENDPOINT environment variable is required")
	}
	// BucketName is optional - we can list buckets if not provided
	// MountPath has a default value, so no need to validate
	return nil
}
