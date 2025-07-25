# Nina

![Nina Logo](nina_lr.png)

[![CI](https://github.com/matiasinsaurralde/nina/workflows/CI/badge.svg)](https://github.com/matiasinsaurralde/nina/actions?query=workflow%3ACI)

A Proof of Concept (PoC) container provisioning engine built with Go, Redis, and Docker Engine.

## Overview

Nina is an experimental container provisioning engine that demonstrates how to build a scalable container orchestration system using modern technologies. The project serves as a research and learning platform for understanding container lifecycle management, distributed systems, and microservices architecture.

## Architecture

Nina leverages three core technologies:

- **Go (Golang)**: High-performance, concurrent programming language for the core engine
- **Redis**: In-memory data structure store for caching, session management, and coordination
- **Docker Engine**: Container runtime for managing container lifecycle operations

## Features

- **Container Provisioning**: Create and manage container deployments via REST API
- **Application Deployment**: Deploy applications from Git repositories with automatic containerization
- **Build System**: Build container images from source code with automatic buildpack detection
- **Reverse Proxy Ingress**: Route HTTP requests based on Host headers to appropriate containers
- **Redis-backed Storage**: Persistent storage for deployment metadata and state
- **RESTful API**: Full CRUD operations for deployments and builds
- **CLI Interface**: Command-line tool for interacting with the API
- **Configurable Logging**: Colored terminal output with multiple log levels
- **XDG-compliant Configuration**: Automatic config file creation in `$HOME/.nina/`

## Prerequisites

- Go 1.24.5 or later
- Docker Engine
- Redis server
- Git

## Installation

1. Clone the repository:
```bash
git clone https://github.com/matiasinsaurralde/nina.git
cd nina
```

2. Install dependencies:
```bash
go mod download
```

3. Set up Redis (if not already running):
```bash
# Using Docker
docker run -d --name redis -p 6379:6379 redis:alpine

# Or install locally
# brew install redis  # macOS
# sudo apt-get install redis-server  # Ubuntu/Debian
```

## Usage

### Starting the Engine Server

```bash
# Start the Engine server with default configuration
./engine

# Start with custom configuration file
./engine -config /path/to/config.json

# Start with verbose logging
./engine -verbose

# Start with custom log level
./engine -log-level debug
```

### Starting the Ingress Proxy

```bash
# Start the ingress proxy
./ingress

# Start with custom configuration
./ingress -config /path/to/config.json -verbose
```

### Using the CLI

```bash
# Check Engine server health
./nina health

# Build a project from the current directory
./nina build

# List all builds
./nina build ls

# Remove builds
./nina build rm [app-name-or-commit-hash]

# Deploy an application from the current directory
./nina deploy

# List all deployments
./nina deploy ls

# Remove a deployment
./nina deploy rm [deployment-id]

# List all deployments (legacy command)
./nina list

# Get deployment status
./nina status <deployment-id>

# Delete a deployment (legacy command)
./nina delete <deployment-id>
```

### API Endpoints

- `GET /health` - Health check
- `POST /api/v1/build` - Create a new build
- `GET /api/v1/builds` - List all builds
- `DELETE /api/v1/builds/:id` - Delete builds by app name or commit hash
- `POST /api/v1/deploy` - Deploy an application
- `GET /api/v1/deployments` - List all deployments
- `GET /api/v1/deployments/:id` - Get deployment by ID
- `GET /api/v1/deployments/:id/status` - Get deployment status
- `DELETE /api/v1/deployments/:id` - Delete a deployment
- `POST /api/v1/provision` - Legacy provisioning endpoint

## Development

### Project Structure

```
nina/
├── cmd/
│   ├── engine/     # Engine server binary
│   ├── ingress/    # Ingress proxy binary
│   └── nina/       # CLI binary
├── pkg/
│   ├── engine/  # Engine server implementation
│   ├── cli/        # CLI client implementation
│   ├── config/     # Configuration management
│   ├── ingress/    # Reverse proxy implementation
│   ├── logger/     # Logging utilities
│   └── store/      # Redis storage layer
├── go.mod          # Go module definition
├── .gitignore      # Git ignore patterns
├── nina_hr.png     # High-resolution project image
├── nina_lr.png     # Low-resolution project image
└── README.md       # This file
```

### Building

```bash
# Build all binaries
go build -o engine ./cmd/engine
go build -o ingress ./cmd/ingress
go build -o nina ./cmd/nina

# Or build everything at once
make build
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with race detection
make test-race

# Run unit tests with Miniredis (no external dependencies)
make test-unit

# Run integration tests (uses real Redis if available, otherwise Miniredis)
make test-integration

# Run specific package tests
go test ./pkg/ingress
go test ./pkg/store
```

### Code Quality

```bash
# Format code
make fmt

# Lint code
make lint

# Run security scan
make security

# Run full CI pipeline locally
make ci
```

## Contributing

This is a PoC project for research and learning purposes. Contributions are welcome for educational and experimental purposes.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

*[License information to be added]*

## Author

- **Matías Insaurralde** - [GitHub](https://github.com/matiasinsaurralde)

## Acknowledgments

- Docker Engine for container runtime capabilities
- Redis for high-performance data storage
- Go community for excellent tooling and ecosystem

## Quick Start

1. **Start Redis** (if not already running):
   ```bash
   docker run -d --name redis -p 6379:6379 redis:alpine
   ```

2. **Build the project**:
   ```bash
   make build
   ```

3. **Start the Engine server**:
   ```bash
   ./engine -verbose
   ```

4. **In another terminal, use the CLI**:
   ```bash
   # Check health
   ./nina health
   
   # Build a project (from a Git repository)
   ./nina build
   
   # Deploy the built project
   ./nina deploy
   
   # List deployments
   ./nina deploy ls
   ```

## Architecture Overview

Nina consists of three main components:

1. **Engine Server** (`cmd/engine`): RESTful API for managing container deployments and builds
2. **Ingress Proxy** (`cmd/ingress`): Reverse proxy that routes requests based on Host headers
3. **CLI Tool** (`cmd/nina`): Command-line interface for interacting with the API

The system uses Redis for persistent storage and supports XDG-compliant configuration management.

## Deployment Workflow

1. **Build**: The `nina build` command creates a container image from your source code
   - Detects the project type (Go, etc.) automatically
   - Creates a Dockerfile if needed
   - Builds and tags the image as `nina-{app-name}-{commit-hash}`

2. **Deploy**: The `nina deploy` command deploys the built application
   - Checks if a build exists for the current commit
   - Creates a deployment record
   - Starts containers using the built image
   - Manages deployment status (unavailable → deploying → ready)

3. **Manage**: Use `nina deploy ls` and `nina deploy rm` to manage deployments

## Continuous Integration

The project includes a comprehensive CI pipeline that runs on every push and pull request:

- **Parallel Builds**: All components (api, ingress, nina) are built in parallel
- **Race Detection**: Tests run with Go's race detector enabled
- **Code Quality**: golangci-lint with multiple linters for code quality
- **Integration Tests**: Redis integration tests with automatic fallback to Miniredis
- **Security Scan**: Vulnerability scanning with govulncheck
- **Validation**: go.mod validation, dependency checks, and formatting validation

The CI workflow ensures code quality and catches issues early in the development process.

---

*This project is a Proof of Concept demonstrating container provisioning concepts. The current implementation provides a complete build and deployment pipeline for containerized applications.* 
