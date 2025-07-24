.PHONY: build clean test run-api run-ingress help

# Binary names
API_BIN=api
INGRESS_BIN=ingress
CLI_BIN=nina

# Build all binaries
build: $(API_BIN) $(INGRESS_BIN) $(CLI_BIN)

# Build API server
$(API_BIN):
	@echo "Building API server..."
	go build -o $(API_BIN) ./cmd/api

# Build ingress proxy
$(INGRESS_BIN):
	@echo "Building ingress proxy..."
	go build -o $(INGRESS_BIN) ./cmd/ingress

# Build CLI
$(CLI_BIN):
	@echo "Building CLI..."
	go build -o $(CLI_BIN) ./cmd/nina

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(API_BIN) $(INGRESS_BIN) $(CLI_BIN)

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -cover ./...

# Run API server
run-api: $(API_BIN)
	@echo "Starting API server..."
	./$(API_BIN) -verbose

# Run ingress proxy
run-ingress: $(INGRESS_BIN)
	@echo "Starting ingress proxy..."
	./$(INGRESS_BIN) -verbose

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run --no-config --disable=errcheck,staticcheck --build-tags=integration

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./pkg/store/...

# Run unit tests with Miniredis
test-unit:
	@echo "Running unit tests with Miniredis..."
	go test -v ./pkg/store/...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -race -v ./...

# Run security scan
security:
	@echo "Running security vulnerability scan..."
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

# CI pipeline (runs all checks)
ci: deps fmt lint test-race test-integration security
	@echo "✅ CI pipeline completed successfully"

# Show help
help:
	@echo "Available targets:"
	@echo "  build        - Build all binaries"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage"
	@echo "  test-integration - Run integration tests"
	@echo "  test-race    - Run tests with race detection"
	@echo "  run-api      - Run API server"
	@echo "  run-ingress  - Run ingress proxy"
	@echo "  deps         - Install dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code"
	@echo "  security     - Run security scan"
	@echo "  ci           - Run full CI pipeline"
	@echo "  help         - Show this help" 