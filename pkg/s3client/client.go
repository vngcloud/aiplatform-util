package s3client

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/vngcloud/aiplatform-util/pkg/config"
)

// Client wraps MinIO client for S3 operations
type Client struct {
	cfg         *config.Config
	minioClient *minio.Client
}

// S3Object represents a file in S3
type S3Object struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
}

// Bucket represents an S3 bucket
type Bucket struct {
	Name         string
	CreationDate time.Time
}

// ProgressReader wraps an io.Reader and reports progress
type ProgressReader struct {
	reader       io.Reader
	total        int64
	current      int64
	key          string
	lastReported int64
}

// NewProgressReader creates a new progress reader
func NewProgressReader(reader io.Reader, total int64, key string) *ProgressReader {
	return &ProgressReader{
		reader: reader,
		total:  total,
		key:    key,
	}
}

// Read implements io.Reader and reports progress
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)

	// Report progress every 10MB or at completion
	if pr.current-pr.lastReported >= 10*1024*1024 || err == io.EOF {
		pr.lastReported = pr.current
		percent := float64(pr.current) / float64(pr.total) * 100
		fmt.Printf("  Progress: %s - %.2f%% (%s / %s)\n",
			pr.key,
			percent,
			formatSize(pr.current),
			formatSize(pr.total))
	}

	return n, err
}

// New creates a new S3 client using MinIO SDK
func New(cfg *config.Config) (*Client, error) {
	// Parse endpoint to remove protocol
	endpoint := strings.TrimPrefix(cfg.Endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	// Determine if using SSL
	useSSL := strings.HasPrefix(cfg.Endpoint, "https://")

	// Initialize MinIO client
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: useSSL,
		Region: "hcm04", // Default region for VNG Cloud
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	return &Client{
		cfg:         cfg,
		minioClient: minioClient,
	}, nil
}

// ListObjects lists all objects in the bucket with optional prefix filter
func (c *Client) ListObjects(ctx context.Context, prefix string, recursive bool) ([]S3Object, error) {
	var objects []S3Object

	// Create list options
	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: recursive,
	}

	// List objects
	objectCh := c.minioClient.ListObjects(ctx, c.cfg.BucketName, opts)
	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("error listing objects: %w", object.Err)
		}

		objects = append(objects, S3Object{
			Key:          object.Key,
			Size:         object.Size,
			LastModified: object.LastModified,
			ETag:         strings.Trim(object.ETag, "\""),
		})
	}

	return objects, nil
}

// DownloadFile downloads a single file from S3 to local path
func (c *Client) DownloadFile(ctx context.Context, key string, localPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Get object info for progress tracking
	objInfo, err := c.minioClient.StatObject(ctx, c.cfg.BucketName, key, minio.StatObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to stat object %s: %w", key, err)
	}

	// Download object
	object, err := c.minioClient.GetObject(ctx, c.cfg.BucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get object %s: %w", key, err)
	}
	defer object.Close()

	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", localPath, err)
	}
	defer localFile.Close()

	// Wrap reader with progress tracking for large files (> 10MB)
	var reader io.Reader = object
	if objInfo.Size > 10*1024*1024 {
		reader = NewProgressReader(object, objInfo.Size, key)
	}

	// Copy with progress
	written, err := io.Copy(localFile, reader)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", key, err)
	}

	// Set modification time to match S3 object
	if err := os.Chtimes(localPath, objInfo.LastModified, objInfo.LastModified); err != nil {
		// Non-fatal error, just log
		fmt.Printf("  Warning: failed to set modification time for %s: %v\n", localPath, err)
	}

	if written != objInfo.Size {
		return fmt.Errorf("size mismatch for %s: expected %d, got %d", key, objInfo.Size, written)
	}

	return nil
}

// UploadFile uploads a single file from local path to S3 with progress tracking
func (c *Client) UploadFile(ctx context.Context, localPath string, key string) error {
	// Get file info
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local file %s: %w", localPath, err)
	}

	// Open local file
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file %s: %w", localPath, err)
	}
	defer file.Close()

	// Wrap reader with progress tracking for large files (> 10MB)
	var reader io.Reader = file
	if fileInfo.Size() > 10*1024*1024 {
		reader = NewProgressReader(file, fileInfo.Size(), key)
	}

	// Determine content type
	contentType := "application/octet-stream"

	// Upload options with 10 concurrent parts for multipart uploads
	// Using smaller part size (16MB) allows more parallel uploads
	uploadOpts := minio.PutObjectOptions{
		ContentType:  contentType,
		NumThreads:   10,                // 10 concurrent uploads for maximum throughput
		PartSize:     16 * 1024 * 1024,  // 16MB part size (more parts = better parallelization)
		SendContentMd5: false,           // Disable MD5 for faster uploads
	}

	// Upload file
	info, err := c.minioClient.PutObject(
		ctx,
		c.cfg.BucketName,
		key,
		reader,
		fileInfo.Size(),
		uploadOpts,
	)
	if err != nil {
		return fmt.Errorf("failed to upload %s: %w", key, err)
	}

	if info.Size != fileInfo.Size() {
		return fmt.Errorf("size mismatch for %s: expected %d, got %d", key, fileInfo.Size(), info.Size)
	}

	return nil
}

// DeleteObject deletes a single object from S3
func (c *Client) DeleteObject(ctx context.Context, key string) error {
	err := c.minioClient.RemoveObject(ctx, c.cfg.BucketName, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", key, err)
	}
	return nil
}

// GetObjectMetadata gets metadata for a single object without downloading it
func (c *Client) GetObjectMetadata(ctx context.Context, key string) (*S3Object, error) {
	objInfo, err := c.minioClient.StatObject(ctx, c.cfg.BucketName, key, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata for %s: %w", key, err)
	}

	return &S3Object{
		Key:          key,
		Size:         objInfo.Size,
		LastModified: objInfo.LastModified,
		ETag:         strings.Trim(objInfo.ETag, "\""),
	}, nil
}

// ListBuckets lists all available S3 buckets
func (c *Client) ListBuckets(ctx context.Context) ([]Bucket, error) {
	buckets, err := c.minioClient.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	var result []Bucket
	for _, bucket := range buckets {
		result = append(result, Bucket{
			Name:         bucket.Name,
			CreationDate: bucket.CreationDate,
		})
	}

	return result, nil
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
