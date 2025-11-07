package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vngcloud/aiplatform-util/pkg/s3client"
)

// PushOptions contains options for push operations
type PushOptions struct {
	Prefix       string
	DryRun       bool
	Delete       bool
	ExcludeGlobs []string
	MountPath    string
}

// PushStats contains statistics about a push operation
type PushStats struct {
	Uploaded int
	Skipped  int
	Deleted  int
	Failed   int
}

// Push syncs files from local workspace to S3
func Push(ctx context.Context, client *s3client.Client, opts PushOptions) (*PushStats, error) {
	stats := &PushStats{}

	// List all objects in S3 (for comparison and deletion)
	remoteObjects, err := client.ListObjects(ctx, opts.Prefix, true)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote objects: %w", err)
	}

	// Build map of remote objects for quick lookup
	remoteFiles := make(map[string]s3client.S3Object)
	for _, obj := range remoteObjects {
		if !strings.HasSuffix(obj.Key, "/") {
			remoteFiles[obj.Key] = obj
		}
	}

	// Walk local directory and upload files
	localFiles := make(map[string]bool)
	prefixPath := filepath.Join(opts.MountPath, opts.Prefix)

	// Check if prefix path exists
	if _, err := os.Stat(prefixPath); os.IsNotExist(err) {
		// If prefix path doesn't exist and we're not deleting, just skip
		if !opts.Delete {
			fmt.Printf("Local path %s does not exist, nothing to push\n", prefixPath)
			return stats, nil
		}
		// If deleting, we still need to process remote deletions
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat prefix path: %w", err)
	} else {
		// Walk the directory
		err = filepath.Walk(prefixPath, func(path string, info os.FileInfo, err error) error {
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

			// Convert to forward slashes for S3 key
			s3Key := filepath.ToSlash(relPath)

			// Check if file should be excluded
			if shouldExclude(s3Key, opts.ExcludeGlobs) {
				return nil
			}

			// Mark as seen
			localFiles[s3Key] = true

			// Check if file needs uploading
			needsUpload, reason := needsUpload(path, info, remoteFiles[s3Key])

			if needsUpload {
				fmt.Printf("Uploading: %s (%s)\n", s3Key, reason)
				if !opts.DryRun {
					if err := client.UploadFile(ctx, path, s3Key); err != nil {
						fmt.Printf("  Failed: %v\n", err)
						stats.Failed++
						return nil
					}
					stats.Uploaded++
				}
			} else {
				if !opts.DryRun {
					stats.Skipped++
				}
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	}

	// Handle deletions if requested
	if opts.Delete {
		for key := range remoteFiles {
			if !localFiles[key] {
				fmt.Printf("Deleting remote: %s (not in local)\n", key)
				if !opts.DryRun {
					if err := client.DeleteObject(ctx, key); err != nil {
						fmt.Printf("  Failed to delete: %v\n", err)
						stats.Failed++
					} else {
						stats.Deleted++
					}
				}
			}
		}
	}

	return stats, nil
}

// needsUpload checks if a file needs to be uploaded
func needsUpload(_ string, localInfo os.FileInfo, remoteObj s3client.S3Object) (bool, string) {
	// If remote doesn't exist, upload
	if remoteObj.Key == "" {
		return true, "new file"
	}

	// Compare size
	if localInfo.Size() != remoteObj.Size {
		return true, "size differs"
	}

	// Compare modification time (with some tolerance)
	// If local is newer, upload
	if localInfo.ModTime().After(remoteObj.LastModified.Add(1 * 1e9)) { // 1 second tolerance
		return true, "local is newer"
	}

	return false, ""
}

// shouldExclude checks if a path matches any exclude patterns
func shouldExclude(path string, patterns []string) bool {
	for _, pattern := range patterns {
		// Simple glob matching (could be enhanced with filepath.Match)
		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}

		// Check if pattern matches as prefix (for directory patterns)
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(path, prefix+"/") {
				return true
			}
		}

		// Exact match
		if path == pattern {
			return true
		}
	}
	return false
}
