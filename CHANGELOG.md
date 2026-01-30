# Changelog

All notable changes to Hydra will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-01-31

### Added

- **Core Download Engine**
  - Multi-connection segmented downloads for faster speeds
  - Automatic segment management and merging
  - Connection pooling and reuse

- **Pause/Resume/Cancel**
  - Full pause and resume support with `.hydra` control files
  - Resume downloads after application restart
  - Cancel individual or all downloads

- **Download Queue**
  - Priority-based queue management
  - Configurable max concurrent downloads
  - Queue position tracking
  - Dynamic concurrency adjustment

- **Session Persistence**
  - Save and restore download state
  - Automatic session recovery
  - JSON-based session files

- **Event System**
  - Real-time download events (start, complete, error, pause, resume, cancel)
  - Progress callbacks with speed and ETA
  - Custom event handlers

- **Checksum Verification**
  - MD5, SHA-1, SHA-256, SHA-512 support
  - Automatic verification on completion
  - Configurable checksum format

- **Speed Control**
  - Per-download and global speed limits
  - Minimum speed threshold with timeout
  - Adaptive connection management

- **CLI Application**
  - Full-featured command-line interface
  - Progress bars and download statistics
  - Batch downloads from file
  - Configurable via flags and environment variables

- **Library API**
  - Clean public API in `pkg/downloader`
  - Functional options pattern
  - Thread-safe Engine for concurrent use
  - Context-based cancellation

### Documentation

- Comprehensive README with quick start guide
- Full CLI reference (`docs/CLI.md`)
- Library API documentation (`docs/LIBRARY.md`)
- Code examples for all features (`docs/EXAMPLES.md`)
- Architecture overview (`docs/ARCHITECTURE.md`)

[Unreleased]: https://github.com/bhunter/hydra/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/bhunter/hydra/releases/tag/v0.1.0
