# aiplatform-util

A fast, efficient command-line tool for managing files between your AI Platform notebook workspace and network volumes (S3-compatible storage).

## Why aiplatform-util?

- **Simple git-like commands** - Familiar `ls`, `pull`, `push` interface
- **Fast uploads** - 10 parallel threads for files larger than 16MB

## Quick Start

### 1. Download

Choose your platform:

```bash
# Linux
curl -L https://github.com/vngcloud/aiplatform-util/releases/latest/download/aiplatform-util-linux-amd64 -o aiplatform-util
chmod +x aiplatform-util
sudo mv aiplatform-util /usr/local/bin/

# macOS
curl -L https://github.com/vngcloud/aiplatform-util/releases/latest/download/aiplatform-util-darwin-amd64 -o aiplatform-util
chmod +x aiplatform-util
sudo mv aiplatform-util /usr/local/bin/
```

### 2. Set Environment Variables

```bash
export AWS_ACCESS_KEY_ID=your_access_key
export AWS_SECRET_ACCESS_KEY=your_secret_key
export AWS_ENDPOINT=https://hcm04.vstorage.vngcloud.vn:443/
export S3_BUCKET=your-bucket-name
export MOUNT_PATH=/workspace/  # Optional, defaults to ~/test/workspace
```

> **Note:** These environment variables are typically pre-configured in AI Platform notebook environments.

### 3. Start Using

```bash
# List files in your network volume
aiplatform-util nv ls

# Download files to your workspace
aiplatform-util nv pull

# Upload your work to network volume
aiplatform-util nv push
```

## Commands

### List Files

Show all files in your network volume:

```bash
aiplatform-util nv ls
```

**Options:**
- `--prefix <path>` - List files under a specific directory
- `--recursive` - List all files recursively (default: true)

**Examples:**
```bash
# List all files
aiplatform-util nv ls

# List files in models directory
aiplatform-util nv ls --prefix models/

# List only top-level items (no recursion)
aiplatform-util nv ls --recursive=false
```

### Pull (Download)

Download files from network volume to your local workspace:

```bash
aiplatform-util nv pull
```

**Options:**
- `--prefix <path>` - Pull only files under a specific directory
- `--dry-run` - Preview what would be downloaded without actually downloading
- `--delete` - Delete local files that don't exist in the network volume

**Examples:**
```bash
# Pull all files
aiplatform-util nv pull

# Pull only the models directory
aiplatform-util nv pull --prefix models/

# See what would be pulled without downloading
aiplatform-util nv pull --dry-run

# Pull and remove local files not in remote
aiplatform-util nv pull --delete
```

### Push (Upload)

Upload files from your workspace to the network volume:

```bash
aiplatform-util nv push
```

**Options:**
- `--prefix <path>` - Push only files under a specific directory
- `--dry-run` - Preview what would be uploaded without actually uploading
- `--delete` - Delete remote files that don't exist locally
- `--exclude <pattern>` - Exclude files matching pattern (can be used multiple times)

**Examples:**
```bash
# Push all files
aiplatform-util nv push

# Push only the outputs directory
aiplatform-util nv push --prefix outputs/

# Push excluding checkpoints
aiplatform-util nv push --exclude "*.ckpt" --exclude "checkpoints/"

# See what would be pushed
aiplatform-util nv push --dry-run

# Push and remove remote files not in local
aiplatform-util nv push --delete
```

### Remove Files

Delete files from the network volume:

```bash
aiplatform-util nv rm <file1> <file2> ...
```

**Options:**
- `--prefix <path>` - Remove all files under a specific directory
- `--recursive` - Remove recursively when using --prefix (default: true)
- `--dry-run` - Preview what would be deleted

**Examples:**
```bash
# Remove a single file
aiplatform-util nv rm myfile.txt

# Remove multiple files
aiplatform-util nv rm file1.txt file2.txt

# Remove all files in a directory
aiplatform-util nv rm --prefix data/

# Preview what would be deleted
aiplatform-util nv rm --prefix data/ --dry-run
```

## Common Workflows

### Starting a New Notebook Session

```bash
# Pull your latest work from network volume
aiplatform-util nv pull
```

### Checking What's Saved

```bash
# List files in network volume
aiplatform-util nv ls
```

### Saving Your Work

```bash
# Push all changes
aiplatform-util nv push

# Or push excluding temporary files
aiplatform-util nv push --exclude "*.tmp" --exclude ".cache/"
```

### Syncing After Deleting Files

```bash
# Push and remove remote files you deleted locally
aiplatform-util nv push --delete

# Pull and remove local files deleted remotely
aiplatform-util nv pull --delete
```

### Preview Before Making Changes

```bash
# See what would be pushed
aiplatform-util nv push --dry-run

# See what would be pulled
aiplatform-util nv pull --dry-run

# See what would be deleted
aiplatform-util nv rm --prefix old-data/ --dry-run
```

## Performance Features

- **Automatic multipart uploads** - Files >16MB are split into chunks and uploaded in parallel
- **10 concurrent threads** - Maximum network throughput for large files
- **Progress tracking** - Real-time progress updates for files >10MB
- **Smart sync** - Only uploads/downloads files that changed
- **Optimized for large files** - Efficient handling of GB-sized model files

## Build from Source

If you prefer to build from source:

```bash
# Clone the repository
git clone https://github.com/vngcloud/aiplatform-util.git
cd aiplatform-util

# Build
go build -o aiplatform-util

# Install
sudo mv aiplatform-util /usr/local/bin/
```

**Requirements:**
- Go 1.21 or higher

## Troubleshooting

### "Access key ID you provided does not exist"

Make sure your environment variables are set correctly:
```bash
echo $AWS_ACCESS_KEY_ID
echo $AWS_SECRET_ACCESS_KEY
echo $AWS_ENDPOINT
```

### "S3_BUCKET environment variable is required"

Set the bucket name:
```bash
export S3_BUCKET=your-bucket-name
```

Or list available buckets:
```bash
aiplatform-util nv ls
```

### Slow Upload Speeds

The tool uses 10 parallel threads for optimal performance. Slow speeds may be due to:
- Network bandwidth limitations
- Server-side throttling
- File system performance

## Compatible Storage

Works with any S3-compatible storage:
- VNG Cloud vStorage
- MinIO
- Amazon S3


## Support

- **Issues:** [GitHub Issues](https://github.com/vngcloud/aiplatform-util/issues)
- **Discussions:** [GitHub Discussions](https://github.com/vngcloud/aiplatform-util/discussions)

## License

MIT License - See [LICENSE](LICENSE) file for details.

## Acknowledgments

Built with:
- [MinIO Go SDK](https://github.com/minio/minio-go) - Fast S3-compatible storage client
- [Cobra](https://github.com/spf13/cobra) - Modern CLI framework
