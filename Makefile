.PHONY: build clean test run-api run-ingress help

# Binary names
ENGINE_BIN=engine
INGRESS_BIN=ingress
CLI_BIN=nina

# Build all binaries
build: $(ENGINE_BIN) $(INGRESS_BIN) $(CLI_BIN)

# Build Engine server
$(ENGINE_BIN):
	@echo "Building Engine server..."
	go build -o $(ENGINE_BIN) ./cmd/engine

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
	rm -f $(ENGINE_BIN) $(INGRESS_BIN) $(CLI_BIN)

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -cover ./...

# Run Engine server
run-engine: $(ENGINE_BIN)
	@echo "Starting Engine server..."
	./$(ENGINE_BIN) -verbose

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
	golangci-lint run --config=.golangci.yml --build-tags=integration

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
	@echo "  run-engine   - Run Engine server"
	@echo "  run-ingress  - Run ingress proxy"
	@echo "  deps         - Install dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code"
	@echo "  security     - Run security scan"
	@echo "  ci           - Run full CI pipeline"
	@echo "  help         - Show this help" 
