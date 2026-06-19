<h1 align="center">go-serve</h1>

A simple and efficient concurrent static file server written in Go

## Features

- High speed and great for concurrent request handling.
- Configurable file cache capacity to improve performance.
- Configurable logging levels (Error, Warn, Info).
- Built-in diagnostics server (pprof) accessible on port 8081.
- Serves Multiple routes for different purposes:
    - `GET /`: For serving static files. Does not require Authorization headers.
    - `POST /`: For creating new files with the payload in request body and adding them to cache. Requires Authorization headers.
    - `PUT /`: For updating existing files with the payload in request body and adding/updating them to cache. Requires Authorization headers.
    - `DELETE /`: For deleting files from disk and cache. Requires Authorization headers.

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
- `-m`: Set system rlimit on Unix systems (default: 0 \[standard system rlimit\]).

Example:
```bash
go-serve -p 3000 -d ./web -c 128 -l Info -m 2048
```

The server will start and output its address to standard error.

### Logging

- Logs are outputted to standard error in the following format:

```
[TIMESTAMP] <METHOD> <PATH>: Status: <STATUS> | Size: <SIZE> | Time: <TIME_TAKEN>
```

- if an error happened while processing the request, logs include an `Error` entry with the error message:

```
[TIMESTAMP]... | Error: <ERROR_MESSAGE>
```

## Development

- `main.go`: Entry point and server configuration.
- `handlers/`: Request handler logic.
- `utils/`: Utilities for logging, caching, and types.
- `sys/`: Utilities that touch the system (syscalls)

See <a href="CONTRIBUTING.md">CONTRIBUTING.md</a> for information about contributing to the project.
See <a href="CHANGELOG.md">CHANGELOG.md</a> for news and changes about the project.

## License

<a href="LICENSE">MIT License</a>
