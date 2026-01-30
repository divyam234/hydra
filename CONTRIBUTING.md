# Contributing to Hydra

Thank you for your interest in contributing to Hydra! This document provides guidelines and information for contributors.

## Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/hydra.git
   cd hydra
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/divyam234/hydra.git
   ```
4. Install dependencies:
   ```bash
   go mod download
   ```

## Development Workflow

### Building

```bash
task build
```

### Running Tests

```bash
# Run all tests
task test

# Run tests with race detector
task test-race

# Run tests with coverage
task test-cover
```

### Linting

```bash
task lint
```

### Formatting

```bash
task fmt
```

## Code Guidelines

### Style

- Follow standard Go conventions and idioms
- Run `go fmt` before committing
- Keep functions focused and small
- Use meaningful variable and function names

### Documentation

- Add godoc comments to all exported types, functions, and methods
- Update relevant documentation in `docs/` when changing functionality
- Include examples for new features

### Testing

- Write tests for new functionality
- Maintain or improve code coverage
- Use table-driven tests where appropriate
- Test edge cases and error conditions

### Commits

- Write clear, concise commit messages
- Use present tense ("Add feature" not "Added feature")
- Reference issues when applicable (`Fixes #123`)

## Pull Request Process

1. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes and commit them

3. Ensure all tests pass:
   ```bash
   task test-race
   task lint
   ```

4. Push to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```

5. Open a Pull Request against the `main` branch

### PR Requirements

- All CI checks must pass
- Code must be formatted with `go fmt`
- New code should include tests
- Documentation should be updated if needed

## Project Structure

```
hydra/
├── cmd/hydra/          # CLI application
├── pkg/                # Public API
│   ├── downloader/     # Main public interface
│   ├── option/         # Configuration options
│   └── apperror/       # Error codes
├── internal/           # Private implementation
│   ├── engine/         # Download engine
│   ├── http/           # HTTP client
│   ├── segment/        # Segment management
│   ├── control/        # Control files
│   ├── stats/          # Statistics
│   ├── limit/          # Rate limiting
│   ├── disk/           # Disk I/O
│   ├── ui/             # Console UI
│   └── util/           # Utilities
└── docs/               # Documentation
```

## Reporting Issues

When reporting issues, please include:

- Hydra version (`hydra --version`)
- Operating system and version
- Go version (`go version`)
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs or error messages

## Feature Requests

Feature requests are welcome! Please:

- Check existing issues to avoid duplicates
- Clearly describe the use case
- Explain why the feature would be valuable

## Questions

For questions about using Hydra, please open a discussion or issue.

## License

By contributing to Hydra, you agree that your contributions will be licensed under the MIT License.
