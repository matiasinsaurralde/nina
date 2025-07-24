# Nina

![Nina Logo](nina_lr.png)

A Proof of Concept (PoC) container provisioning engine built with Go, Redis, and Docker Engine.

## Overview

Nina is an experimental container provisioning engine that demonstrates how to build a scalable container orchestration system using modern technologies. The project serves as a research and learning platform for understanding container lifecycle management, distributed systems, and microservices architecture.

## Architecture

Nina leverages three core technologies:

- **Go (Golang)**: High-performance, concurrent programming language for the core engine
- **Redis**: In-memory data structure store for caching, session management, and coordination
- **Docker Engine**: Container runtime for managing container lifecycle operations

## Features

*[Features to be documented as the project evolves]*

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

*[Usage instructions to be added as the project develops]*

## Development

### Project Structure

```
nina/
├── go.mod          # Go module definition
├── .gitignore      # Git ignore patterns
├── nina_hr.png     # High-resolution project image
├── nina_lr.png     # Low-resolution project image
└── README.md       # This file
```

### Building

```bash
go build -o nina .
```

### Running Tests

```bash
go test ./...
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

---

*This project is currently in development as a Proof of Concept. More details and documentation will be added as the project evolves.* 
