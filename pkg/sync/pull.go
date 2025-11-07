package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vngcloud/aiplatform-util/pkg/s3client"
)

// PullOptions contains options for pull operations
type PullOptions struct {
	Prefix    string
	DryRun    bool
	Delete    bool
	MountPath string
}

// PullStats contains statistics about a pull operation
type PullStats struct {
	Downloaded int
	Skipped    int
	Deleted    int
	Failed     int
}

// Pull syncs files from S3 to local workspace
func Pull(ctx context.Context, client *s3client.Client, opts PullOptions) (*PullStats, error) {
	stats := &PullStats{}

	// List all objects in S3
	objects, err := client.ListObjects(ctx, opts.Prefix, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	// Create mount path if it doesn't exist
	if !opts.DryRun {
		if err := os.MkdirAll(opts.MountPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create mount path %s: %w", opts.MountPath, err)
		}
	}

	// Download files that need updating
	for _, obj := range objects {
		// Skip directories
		if strings.HasSuffix(obj.Key, "/") {
			continue
		}

		localPath := filepath.Join(opts.MountPath, obj.Key)

		// Check if local file exists and is up to date
		needsDownload, reason := needsDownload(obj, localPath)

		if needsDownload {
			fmt.Printf("Downloading: %s (%s)\n", obj.Key, reason)
			if !opts.DryRun {
				if err := client.DownloadFile(ctx, obj.Key, localPath); err != nil {
					fmt.Printf("  Failed: %v\n", err)
					stats.Failed++
					continue
				}
				stats.Downloaded++
			}
		} else {
			if !opts.DryRun {
				stats.Skipped++
			}
		}
	}

	// Handle deletions if requested
	if opts.Delete {
		// Build set of remote keys for quick lookup
		remoteKeys := make(map[string]bool)
		for _, obj := range objects {
			if !strings.HasSuffix(obj.Key, "/") {
				remoteKeys[obj.Key] = true
			}
		}

		// Walk local directory and find files to delete
		prefixPath := filepath.Join(opts.MountPath, opts.Prefix)
		err := filepath.Walk(prefixPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories
			if info.IsDir() {
				return nil
			}

			// Get relative path
			relPath, err := filepath.Rel(opts.MountPath, path)
			if err != nil {
				return err
			}

			// Convert to forward slashes for S3 key comparison
			relPath = filepath.ToSlash(relPath)

			// Check if file exists in remote
			if !remoteKeys[relPath] {
				fmt.Printf("Deleting local: %s (not in remote)\n", relPath)
				if !opts.DryRun {
					if err := os.Remove(path); err != nil {
						fmt.Printf("  Failed to delete: %v\n", err)
						stats.Failed++
					} else {
						stats.Deleted++
					}
				}
			}

			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	}

	return stats, nil
}

// needsDownload checks if a file needs to be downloaded
func needsDownload(obj s3client.S3Object, localPath string) (bool, string) {
	info, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, "new file"
		}
		return true, "stat error"
	}

	// Compare size
	if info.Size() != obj.Size {
		return true, "size differs"
	}

	// Compare modification time (with some tolerance for filesystem differences)
	// If remote is newer, download
	if obj.LastModified.After(info.ModTime().Add(1 * 1e9)) { // 1 second tolerance
		return true, "remote is newer"
	}

	return false, ""
}
