# Hydra CLI Reference

Complete command-line interface documentation for Hydra.

## Table of Contents

- [Installation](#installation)
- [Commands](#commands)
- [Global Options](#global-options)
- [Download Options](#download-options)
- [Examples](#examples)
- [Environment Variables](#environment-variables)
- [Exit Codes](#exit-codes)

## Installation

```bash
# Install from Go
go install github.com/divyam234/hydra/cmd/hydra@latest

# Or build from source
git clone https://github.com/divyam234/hydra.git
cd hydra
go build -o hydra ./cmd/hydra
```

## Commands

### hydra

The root command. Shows help information.

```bash
hydra --help
```

**Output:**
```
Hydra is a high-performance, multi-connection download manager written in Go.

Usage:
  hydra [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  download    Download files from URLs
  help        Help about any command

Flags:
  -h, --help   help for hydra

Use "hydra [command] --help" for more information about a command.
```

### hydra download

Download files from one or more URLs.

```bash
hydra download [urls...] [flags]
```

## Download Options

### Connection Options

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--split` | `-s` | int | 5 | Number of connections per download |
| `--timeout` | | int | 60 | Timeout in seconds |
| `--connect-timeout` | | int | 15 | Connection timeout in seconds |
| `--max-tries` | | int | 5 | Number of retry attempts |
| `--retry-wait` | | int | 0 | Wait time between retries (seconds) |

### Speed Control

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--max-download-limit` | string | 0 (unlimited) | Max download speed (e.g., `1M`, `500K`) |
| `--lowest-speed-limit` | string | 0 (disabled) | Minimum speed before reconnect (e.g., `10K`) |

**Speed format:**
- `K` or `k` = Kilobytes per second
- `M` or `m` = Megabytes per second
- Plain number = Bytes per second

### Output Options

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Download directory |
| `--out` | `-o` | string | URL filename | Output filename |

### HTTP Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--user-agent` | string | `hydra/0.1.0` | User-Agent header |
| `--referer` | string | | Referer header |
| `--header` | string[] | | Custom headers (repeatable) |

### Authentication

| Flag | Type | Description |
|------|------|-------------|
| `--http-user` | string | HTTP Basic Auth username |
| `--http-passwd` | string | HTTP Basic Auth password |
| `--load-cookies` | string | Path to Netscape/Mozilla cookie file |

### Proxy Options

| Flag | Type | Description |
|------|------|-------------|
| `--http-proxy` | string | HTTP proxy URL |
| `--https-proxy` | string | HTTPS proxy URL |
| `--all-proxy` | string | Proxy for all protocols |

### Verification

| Flag | Type | Description |
|------|------|-------------|
| `--checksum` | string | Verify checksum after download |

**Checksum format:** `algorithm=hash`

Supported algorithms:
- `md5`
- `sha-1`
- `sha-256`
- `sha-512`

### Advanced Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--max-pieces-per-segment` | int | 20 | Chunk size control |

## Examples

### Basic Download

```bash
# Download a file
hydra download "https://example.com/file.zip"

# Download multiple files
hydra download "https://example.com/file1.zip" "https://example.com/file2.zip"
```

### Multi-Connection Download

```bash
# Use 8 connections for faster download
hydra download "https://example.com/large.iso" --split 8

# Use 16 connections
hydra download "https://example.com/huge.tar.gz" -s 16
```

### Output Control

```bash
# Save to specific directory
hydra download "https://example.com/file.zip" -d /tmp/downloads

# Save with custom filename
hydra download "https://example.com/file.zip" -o myfile.zip

# Both directory and filename
hydra download "https://example.com/file.zip" -d /tmp -o custom-name.zip
```

### Speed Limiting

```bash
# Limit to 5 MB/s
hydra download "https://example.com/file.zip" --max-download-limit 5M

# Limit to 500 KB/s
hydra download "https://example.com/file.zip" --max-download-limit 500K

# Reconnect if speed drops below 10 KB/s
hydra download "https://example.com/file.zip" --lowest-speed-limit 10K
```

### Retry Configuration

```bash
# Retry up to 10 times
hydra download "https://example.com/file.zip" --max-tries 10

# Wait 5 seconds between retries
hydra download "https://example.com/file.zip" --max-tries 10 --retry-wait 5
```

### Timeout Configuration

```bash
# Set connection timeout to 30 seconds
hydra download "https://example.com/file.zip" --connect-timeout 30

# Set overall timeout to 120 seconds
hydra download "https://example.com/file.zip" --timeout 120
```

### Custom Headers

```bash
# Add single header
hydra download "https://example.com/file.zip" --header "Authorization: Bearer token123"

# Add multiple headers
hydra download "https://example.com/file.zip" \
  --header "Authorization: Bearer token123" \
  --header "X-Custom-Header: value"

# Set custom User-Agent
hydra download "https://example.com/file.zip" --user-agent "Mozilla/5.0"

# Set Referer
hydra download "https://example.com/file.zip" --referer "https://example.com/"
```

### Authentication

```bash
# HTTP Basic Auth
hydra download "https://example.com/protected/file.zip" \
  --http-user myuser \
  --http-passwd mypassword

# Cookie-based authentication
hydra download "https://example.com/protected/file.zip" \
  --load-cookies cookies.txt
```

**Cookie file format (Netscape/Mozilla):**
```
# Netscape HTTP Cookie File
.example.com	TRUE	/	FALSE	0	session_id	abc123
.example.com	TRUE	/	FALSE	0	auth_token	xyz789
```

### Proxy Usage

```bash
# HTTP proxy
hydra download "https://example.com/file.zip" --http-proxy "http://proxy:8080"

# HTTPS proxy
hydra download "https://example.com/file.zip" --https-proxy "http://proxy:8080"

# Proxy for all protocols
hydra download "https://example.com/file.zip" --all-proxy "http://proxy:8080"

# Proxy with authentication
hydra download "https://example.com/file.zip" \
  --all-proxy "http://user:pass@proxy:8080"
```

### Checksum Verification

```bash
# Verify SHA-256 checksum
hydra download "https://example.com/file.zip" \
  --checksum "sha-256=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

# Verify MD5 checksum
hydra download "https://example.com/file.zip" \
  --checksum "md5=d41d8cd98f00b204e9800998ecf8427e"

# Verify SHA-1 checksum
hydra download "https://example.com/file.zip" \
  --checksum "sha-1=da39a3ee5e6b4b0d3255bfef95601890afd80709"
```

### Complete Example

```bash
# Full-featured download
hydra download "https://example.com/large-file.iso" \
  --dir /home/user/downloads \
  --out ubuntu-22.04.iso \
  --split 8 \
  --max-download-limit 10M \
  --lowest-speed-limit 100K \
  --max-tries 5 \
  --retry-wait 3 \
  --user-agent "Mozilla/5.0" \
  --checksum "sha-256=abc123..."
```

## Environment Variables

Hydra respects the following environment variables for proxy configuration:

| Variable | Description |
|----------|-------------|
| `HTTP_PROXY` | HTTP proxy URL |
| `HTTPS_PROXY` | HTTPS proxy URL |
| `ALL_PROXY` | Proxy for all protocols |
| `NO_PROXY` | Comma-separated list of hosts to bypass proxy |

Command-line flags take precedence over environment variables.

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | General error (download failed, invalid arguments, etc.) |

## Signal Handling

Hydra handles the following signals gracefully:

| Signal | Behavior |
|--------|----------|
| `SIGINT` (Ctrl+C) | Save state and exit gracefully |
| `SIGTERM` | Save state and exit gracefully |

When interrupted, Hydra saves the download progress to a `.hydra` control file, allowing the download to resume later.

## Resume Downloads

Hydra automatically resumes interrupted downloads:

```bash
# Start a download
hydra download "https://example.com/large.iso"
# Press Ctrl+C to interrupt

# Resume the same download
hydra download "https://example.com/large.iso"
# Hydra automatically detects the .hydra control file and resumes
```

The control file (`filename.hydra`) contains:
- Download progress (bitfield)
- Original URL(s)
- File metadata

## Shell Completion

Generate shell completion scripts:

```bash
# Bash
hydra completion bash > /etc/bash_completion.d/hydra

# Zsh
hydra completion zsh > "${fpath[1]}/_hydra"

# Fish
hydra completion fish > ~/.config/fish/completions/hydra.fish

# PowerShell
hydra completion powershell > hydra.ps1
```
