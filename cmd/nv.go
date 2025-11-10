package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vngcloud/aiplatform-util/pkg/config"
	"github.com/vngcloud/aiplatform-util/pkg/s3client"
	"github.com/vngcloud/aiplatform-util/pkg/sync"
)

// nvCmd represents the nv (network volume) command
var nvCmd = &cobra.Command{
	Use:   "nv",
	Short: "Manage network volume (S3 storage)",
	Long: `Network volume commands for listing, pulling, and pushing files between
your local workspace and the S3-compatible network volume.

Available commands:
  ls    - List files in the network volume
  pull  - Pull files from network volume to local workspace
  push  - Push files from local workspace to network volume`,
}

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List files in the network volume",
	Long: `List all files in the network volume (S3 bucket) with their sizes and modification times.

Examples:
  aiplatform-util nv ls
  aiplatform-util nv ls --prefix models/
  aiplatform-util nv ls --prefix data/ --recursive`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Create S3 client
		client, err := s3client.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to create S3 client: %w", err)
		}

		// If no bucket specified, list available buckets
		if cfg.BucketName == "" {
			buckets, err := client.ListBuckets(ctx)
			if err != nil {
				return fmt.Errorf("failed to list buckets: %w", err)
			}
			fmt.Println("Available buckets (set S3_BUCKET via /etc/config-nv/S3_BUCKET file or environment variable to select one):")
			for _, bucket := range buckets {
				fmt.Printf("  - %s (created: %s)\n", bucket.Name, bucket.CreationDate.Format("2006-01-02 15:04:05"))
			}
			return nil
		}

		// Get flags
		prefix, _ := cmd.Flags().GetString("prefix")
		recursive, _ := cmd.Flags().GetBool("recursive")

		// List objects
		objects, err := client.ListObjects(ctx, prefix, recursive)
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		if len(objects) == 0 {
			fmt.Println("No objects found")
			return nil
		}

		// Print header
		fmt.Printf("Listing objects in bucket: %s\n", cfg.BucketName)
		if prefix != "" {
			fmt.Printf("Prefix: %s\n", prefix)
		}
		fmt.Println()
		fmt.Printf("%-60s %15s %25s\n", "KEY", "SIZE", "LAST MODIFIED")
		fmt.Println("─────────────────────────────────────────────────────────────────────────────────────────────────────")

		// Print objects
		for _, obj := range objects {
			sizeStr := formatSize(obj.Size)
			modifiedStr := ""
			if !obj.LastModified.IsZero() {
				modifiedStr = obj.LastModified.Format("2006-01-02 15:04:05")
			}

			// Mark directories with trailing /
			key := obj.Key
			if obj.Size == 0 && obj.ETag == "" && obj.LastModified.IsZero() {
				fmt.Printf("%-60s %15s %25s\n", key, "<DIR>", "")
			} else {
				fmt.Printf("%-60s %15s %25s\n", key, sizeStr, modifiedStr)
			}
		}

		fmt.Printf("\nTotal: %d objects\n", len(objects))
		return nil
	},
}

// pullCmd represents the pull command
var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull files from network volume to local workspace",
	Long: `Download files from the network volume (S3 bucket) to your local workspace.
Only downloads new or modified files by comparing timestamps and sizes.

Examples:
  aiplatform-util nv pull
  aiplatform-util nv pull --prefix models/
  aiplatform-util nv pull --dry-run
  aiplatform-util nv pull --delete`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Check bucket name is set
		if cfg.BucketName == "" {
			return fmt.Errorf("S3_BUCKET is required for pull operations (set via /etc/config-nv/S3_BUCKET file or environment variable)")
		}

		// Create S3 client
		client, err := s3client.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to create S3 client: %w", err)
		}

		// Get flags
		prefix, _ := cmd.Flags().GetString("prefix")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		deleteLocal, _ := cmd.Flags().GetBool("delete")

		// Print operation info
		fmt.Printf("Pulling from bucket: %s to %s\n", cfg.BucketName, cfg.MountPath)
		if prefix != "" {
			fmt.Printf("Prefix: %s\n", prefix)
		}
		if dryRun {
			fmt.Println("DRY RUN - no changes will be made")
		}
		fmt.Println()

		// Perform pull
		stats, err := sync.Pull(ctx, client, sync.PullOptions{
			Prefix:    prefix,
			DryRun:    dryRun,
			Delete:    deleteLocal,
			MountPath: cfg.MountPath,
		})
		if err != nil {
			return fmt.Errorf("pull failed: %w", err)
		}

		// Print summary
		fmt.Println()
		fmt.Println("─────────────────────────────────────")
		if dryRun {
			fmt.Println("Summary (dry run):")
		} else {
			fmt.Println("Summary:")
		}
		fmt.Printf("  Downloaded: %d files\n", stats.Downloaded)
		fmt.Printf("  Skipped:    %d files (already up to date)\n", stats.Skipped)
		if deleteLocal {
			fmt.Printf("  Deleted:    %d files\n", stats.Deleted)
		}
		if stats.Failed > 0 {
			fmt.Printf("  Failed:     %d files\n", stats.Failed)
		}
		fmt.Println("─────────────────────────────────────")

		return nil
	},
}

// pushCmd represents the push command
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push files from local workspace to network volume",
	Long: `Upload files from your local workspace to the network volume (S3 bucket).
Only uploads new or modified files by comparing timestamps and sizes.

Examples:
  aiplatform-util nv push
  aiplatform-util nv push --prefix models/
  aiplatform-util nv push --dry-run
  aiplatform-util nv push --delete
  aiplatform-util nv push --exclude "*.tmp" --exclude ".git/*"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Check bucket name is set
		if cfg.BucketName == "" {
			return fmt.Errorf("S3_BUCKET is required for push operations (set via /etc/config-nv/S3_BUCKET file or environment variable)")
		}

		// Create S3 client
		client, err := s3client.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to create S3 client: %w", err)
		}

		// Get flags
		prefix, _ := cmd.Flags().GetString("prefix")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		deleteRemote, _ := cmd.Flags().GetBool("delete")
		exclude, _ := cmd.Flags().GetStringSlice("exclude")

		// Print operation info
		fmt.Printf("Pushing from %s to bucket: %s\n", cfg.MountPath, cfg.BucketName)
		if prefix != "" {
			fmt.Printf("Prefix: %s\n", prefix)
		}
		if len(exclude) > 0 {
			fmt.Printf("Exclude patterns: %v\n", exclude)
		}
		if dryRun {
			fmt.Println("DRY RUN - no changes will be made")
		}
		fmt.Println()

		// Perform push
		stats, err := sync.Push(ctx, client, sync.PushOptions{
			Prefix:       prefix,
			DryRun:       dryRun,
			Delete:       deleteRemote,
			ExcludeGlobs: exclude,
			MountPath:    cfg.MountPath,
		})
		if err != nil {
			return fmt.Errorf("push failed: %w", err)
		}

		// Print summary
		fmt.Println()
		fmt.Println("─────────────────────────────────────")
		if dryRun {
			fmt.Println("Summary (dry run):")
		} else {
			fmt.Println("Summary:")
		}
		fmt.Printf("  Uploaded:  %d files\n", stats.Uploaded)
		fmt.Printf("  Skipped:   %d files (already up to date)\n", stats.Skipped)
		if deleteRemote {
			fmt.Printf("  Deleted:   %d files\n", stats.Deleted)
		}
		if stats.Failed > 0 {
			fmt.Printf("  Failed:    %d files\n", stats.Failed)
		}
		fmt.Println("─────────────────────────────────────")

		return nil
	},
}

// rmCmd represents the rm (remove) command
var rmCmd = &cobra.Command{
	Use:   "rm [key...]",
	Short: "Remove files from network volume",
	Long: `Remove one or more files from the network volume (S3 bucket).

Examples:
  aiplatform-util nv rm myfile.txt
  aiplatform-util nv rm file1.txt file2.txt
  aiplatform-util nv rm models/model.pth
  aiplatform-util nv rm --prefix data/  # Remove all files under data/
  aiplatform-util nv rm --prefix data/ --dry-run  # Preview what would be deleted`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Check bucket name is set
		if cfg.BucketName == "" {
			return fmt.Errorf("S3_BUCKET is required for remove operations (set via /etc/config-nv/S3_BUCKET file or environment variable)")
		}

		// Create S3 client
		client, err := s3client.New(cfg)
		if err != nil {
			return fmt.Errorf("failed to create S3 client: %w", err)
		}

		// Get flags
		prefix, _ := cmd.Flags().GetString("prefix")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		recursive, _ := cmd.Flags().GetBool("recursive")

		var keysToDelete []string

		// If prefix is provided, list and delete all files under that prefix
		if prefix != "" {
			objects, err := client.ListObjects(ctx, prefix, recursive)
			if err != nil {
				return fmt.Errorf("failed to list objects: %w", err)
			}

			for _, obj := range objects {
				// Skip directories
				if !strings.HasSuffix(obj.Key, "/") {
					keysToDelete = append(keysToDelete, obj.Key)
				}
			}
		} else if len(args) == 0 {
			return fmt.Errorf("either provide file keys as arguments or use --prefix flag")
		} else {
			// Use provided arguments as keys
			keysToDelete = args
		}

		if len(keysToDelete) == 0 {
			fmt.Println("No files to delete")
			return nil
		}

		// Print operation info
		fmt.Printf("Removing from bucket: %s\n", cfg.BucketName)
		if dryRun {
			fmt.Println("DRY RUN - no changes will be made")
		}
		fmt.Println()

		// Delete files
		deleted := 0
		failed := 0

		for _, key := range keysToDelete {
			fmt.Printf("Deleting: %s\n", key)
			if !dryRun {
				if err := client.DeleteObject(ctx, key); err != nil {
					fmt.Printf("  Failed: %v\n", err)
					failed++
				} else {
					deleted++
				}
			}
		}

		// Print summary
		fmt.Println()
		fmt.Println("─────────────────────────────────────")
		if dryRun {
			fmt.Printf("Summary (dry run): %d files would be deleted\n", len(keysToDelete))
		} else {
			fmt.Println("Summary:")
			fmt.Printf("  Deleted: %d files\n", deleted)
			if failed > 0 {
				fmt.Printf("  Failed:  %d files\n", failed)
			}
		}
		fmt.Println("─────────────────────────────────────")

		return nil
	},
}

// formatSize formats bytes as human-readable string
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	if bytes == 0 {
		return "0 B"
	}

	switch {
	case bytes < KB:
		return fmt.Sprintf("%d B", bytes)
	case bytes < MB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	case bytes < GB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes < TB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	default:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	}
}

func init() {
	// Add nv command to root
	rootCmd.AddCommand(nvCmd)

	// Add subcommands to nv
	nvCmd.AddCommand(lsCmd)
	nvCmd.AddCommand(pullCmd)
	nvCmd.AddCommand(pushCmd)
	nvCmd.AddCommand(rmCmd)

	// Flags for ls command
	lsCmd.Flags().String("prefix", "", "Filter by prefix/directory")
	lsCmd.Flags().Bool("recursive", true, "List recursively")

	// Flags for pull command
	pullCmd.Flags().String("prefix", "", "Pull only specific prefix")
	pullCmd.Flags().Bool("dry-run", false, "Preview without executing")
	pullCmd.Flags().Bool("delete", false, "Delete local files not in remote")

	// Flags for push command
	pushCmd.Flags().String("prefix", "", "Push only specific prefix")
	pushCmd.Flags().Bool("dry-run", false, "Preview without executing")
	pushCmd.Flags().Bool("delete", false, "Delete remote files not in local")
	pushCmd.Flags().StringSlice("exclude", []string{}, "Exclude patterns (can be repeated)")

	// Flags for rm command
	rmCmd.Flags().String("prefix", "", "Remove all files under this prefix")
	rmCmd.Flags().Bool("dry-run", false, "Preview without executing")
	rmCmd.Flags().Bool("recursive", true, "Remove recursively when using --prefix")
}
