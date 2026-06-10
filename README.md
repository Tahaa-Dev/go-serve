<h1 align="center">go-serve</h1>

A simple and efficient concurrent static file server written in Go.

## Features

- High speed and great for concurrent request handling.
- Serves static files from a specified directory.
- Customizable server port.
- Configurable file cache capacity to improve performance.
- Configurable logging levels (Error, Warn, Info).
- Built-in diagnostics server (pprof) accessible on port 8081.

## Installation

1. Ensure you have Go installed on your system.
2. Build the project:
- from root directory:
   ```bash
   go build -o go-serve .
   ```

- or using `go install`:
   ```bash
   go install github.com/Tahaa-Dev/go-serve@latest
   ```

## Usage

You can run the server using the generated binary:

```bash
go-serve [options]
```

### Options

- `-p`: Set the port (default: 8000).
- `-d`: Set the directory to serve (default: .).
- `-c`: Set the cache entry limit (default: 64).
- `-l`: Set the log level threshold (options: Error, Warn, Info; default: Warn).

Example:
```bash
go-serve -p 3000 -d ./web -c 128 -l Info
```

The server will start and output its address to standard error.

## Development

- `main.go`: Entry point and server configuration.
- `handlers/`: Request handler logic.
- `utils/`: Utilities for logging, caching, and types.

See <a href="CONTRIBUTING.md">CONTRIBUTING.md</a> for information about contributing to the project.
See <a href="CHANGELOG.md">CHANGELOG.md</a> for news and changes about the project.

## License

<a href="LICENSE">MIT License</a>
