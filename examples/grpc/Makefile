# Makefile for gRPC example
# Author: NetCore-Go Team
# Created: 2024

.PHONY: help generate clean build run-server run-client test install-deps

# Default target
help:
	@echo "Available targets:"
	@echo "  generate     - Generate protobuf code from .proto files"
	@echo "  clean        - Clean generated files"
	@echo "  build        - Build server and client binaries"
	@echo "  run-server   - Run the gRPC server"
	@echo "  run-client   - Run the gRPC client"
	@echo "  test         - Run tests"
	@echo "  install-deps - Install required dependencies"
	@echo "  help         - Show this help message"

# Generate protobuf code
generate:
	@echo "Generating protobuf code..."
	@go run generate.go

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	@rm -f proto/*.pb.go
	@rm -f server/server
	@rm -f client/client

# Install dependencies
install-deps:
	@echo "Installing protobuf dependencies..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Dependencies installed successfully!"
	@echo ""
	@echo "Make sure you also have protoc installed:"
	@echo "  - Download from: https://github.com/protocolbuffers/protobuf/releases"
	@echo "  - Or install via package manager (e.g., brew install protobuf)"

# Build binaries
build: generate
	@echo "Building server..."
	@cd server && go build -o server main.go
	@echo "Building client..."
	@cd client && go build -o client main.go
	@echo "Build completed!"

# Run server
run-server: build
	@echo "Starting gRPC server..."
	@cd server && ./server

# Run client
run-client: build
	@echo "Starting gRPC client..."
	@cd client && ./client

# Run tests
test: generate
	@echo "Running tests..."
	@go test -v ./...

# Development workflow
dev: clean install-deps generate build
	@echo "Development setup completed!"
	@echo "You can now run 'make run-server' and 'make run-client'"

# Check protoc installation
check-protoc:
	@echo "Checking protoc installation..."
	@which protoc > /dev/null || (echo "protoc not found! Please install Protocol Buffers compiler." && exit 1)
	@protoc --version
	@echo "protoc is installed!"

# Full setup from scratch
setup: check-protoc install-deps dev
	@echo "Full setup completed!"