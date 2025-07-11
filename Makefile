# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

.PHONY: all clean test test-verbose deps fmt help

# Default target
all: clean deps fmt test

# Clean build artifacts
clean:
	$(GOCLEAN)

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Install/update dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
fmt:
	$(GOFMT) ./...

# Lint the code (requires golangci-lint)
lint:
	golangci-lint run

# Generate mock functions for testing (requires gomock)
mocks:
	@echo "Generating mocks..."
	$(GOCMD) generate ./...

# Development setup
dev-setup:
	$(GOGET) -u github.com/golangci/golangci-lint/cmd/golangci-lint
	$(GOGET) -u github.com/golang/mock/mockgen

# Help target
help:
	@echo "Available targets:"
	@echo "  all          - Clean, deps, fmt, and test"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint the code"
	@echo "  keygen       - Generate a Solana keypair"
	@echo "  keygen-multi - Generate multiple Solana keypairs"
	@echo "  keygen-json  - Generate keypairs in JSON format"
	@echo "  keygen-env   - Generate keypairs in environment format"
	@echo "  dev-setup    - Install development tools"
	@echo "  help         - Show this help message"

# Generate Solana keypairs
keygen:
	$(GOCMD) run cmd/keygen/main.go

# Generate multiple keypairs
keygen-multi:
	$(GOCMD) run cmd/keygen/main.go -count=5

# Generate keypairs in JSON format
keygen-json:
	$(GOCMD) run cmd/keygen/main.go -format=json

# Generate keypairs in environment format
keygen-env:
	$(GOCMD) run cmd/keygen/main.go -format=env 