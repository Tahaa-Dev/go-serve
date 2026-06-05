# Contributing to go-serve

## Requirements

- `go` version `1.26.3`
- `golangci-lint` version 2+
- `golangci/golines` (optional, can use `golangci-lint fmt` instead)

## Quick Start

1. Fork the repository
2. Create a branch: `git checkout -b feature/your-feature`
3. Make your changes
4. Run tests: `go test ./tests`
5. Run lints: `golangci-lint run`
6. Format code: `golines -w .` or `golangci-lint fmt` if you don't have `golangci/golines` installed.
7. Commit: `git commit -m "Add feature X"`
8. Push: `git push origin feature/your-feature`
9. Open a Pull Request

## Guidelines

### Code Style

- Follow Go standard style enforced by `golines` (which uses gofmt as a formatter then breaks long lines)
- Run `golangci-lint run` and fix all warnings
- Run tests with `go test ./tests` and make sure they pass
- Add tests for new features in ./tests directory

### Commit Messages

- Be descriptive: "Fix: Fix possible lock contention in req.go:68" not "Update code"
- Reference issues: "Fix #123: Handle empty request paths"

### Pull Requests

- Keep PRs focused (one feature/fix per PR)
- Include tests for new functionality
- Update README if adding user-facing features
- Describe what changed and why

### Reporting Issues

- Check existing issues first
- Provide minimal reproduction case
- Use markdown code blocks for code/errors

### Questions?

Open an issue or discussion. We're happy to help!

### License

By contributing, you agree your contributions will be licensed under MIT.
