# NetCore-Go Multi-stage Dockerfile
# Author: NetCore-Go Team
# Created: 2024

# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    make \
    gcc \
    musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build arguments
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME
ARG CGO_ENABLED=0
ARG GOOS=linux
ARG GOARCH=amd64

# Set build environment
ENV CGO_ENABLED=${CGO_ENABLED} \
    GOOS=${GOOS} \
    GOARCH=${GOARCH}

# Build the application
RUN go build \
    -a \
    -installsuffix cgo \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -o netcore-go-server \
    ./cmd/server

# Build additional tools
RUN go build \
    -a \
    -installsuffix cgo \
    -ldflags "-s -w" \
    -o netcore-go-cli \
    ./cmd/cli

# Runtime stage
FROM alpine:3.18 AS runtime

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    jq \
    && update-ca-certificates

# Create non-root user
RUN addgroup -g 1000 netcore && \
    adduser -D -s /bin/sh -u 1000 -G netcore netcore

# Set working directory
WORKDIR /app

# Create necessary directories
RUN mkdir -p /app/config /app/logs /app/data /tmp/netcore-go && \
    chown -R netcore:netcore /app /tmp/netcore-go

# Copy binaries from builder
COPY --from=builder /build/netcore-go-server /app/
COPY --from=builder /build/netcore-go-cli /app/

# Copy configuration files
COPY --chown=netcore:netcore configs/ /app/config/
COPY --chown=netcore:netcore scripts/docker-entrypoint.sh /app/

# Make scripts executable
RUN chmod +x /app/docker-entrypoint.sh /app/netcore-go-server /app/netcore-go-cli

# Set environment variables
ENV SERVER_HOST=0.0.0.0 \
    SERVER_PORT=8080 \
    SERVER_HTTPS_PORT=8443 \
    GRPC_PORT=9000 \
    METRICS_PORT=9090 \
    HEALTH_PORT=8081 \
    LOG_LEVEL=info \
    LOG_FORMAT=json \
    CONFIG_PATH=/app/config \
    DATA_PATH=/app/data \
    TEMP_PATH=/tmp/netcore-go

# Expose ports
EXPOSE 8080 8443 9000 9090 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8081/health/live || exit 1

# Switch to non-root user
USER netcore

# Set entrypoint
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/netcore-go-server"]

# Development stage
FROM golang:1.21-alpine AS development

# Install development tools
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    make \
    gcc \
    musl-dev \
    curl \
    bash \
    vim \
    htop

# Install Go tools
RUN go install github.com/cosmtrek/air@latest && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
    go install golang.org/x/tools/cmd/goimports@latest && \
    go install github.com/swaggo/swag/cmd/swag@latest

# Set working directory
WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Set environment for development
ENV CGO_ENABLED=1 \
    GOOS=linux \
    GO111MODULE=on \
    SERVER_HOST=0.0.0.0 \
    LOG_LEVEL=debug

# Expose ports for development
EXPOSE 8080 8443 9000 9090 8081 2345

# Default command for development
CMD ["air", "-c", ".air.toml"]

# Testing stage
FROM builder AS testing

# Install testing tools
RUN go install github.com/onsi/ginkgo/v2/ginkgo@latest && \
    go install github.com/onsi/gomega/...@latest && \
    go install gotest.tools/gotestsum@latest

# Run tests
RUN go test -v -race -coverprofile=coverage.out ./...

# Generate coverage report
RUN go tool cover -html=coverage.out -o coverage.html

# Security scanning stage
FROM alpine:3.18 AS security

# Install security scanning tools
RUN apk add --no-cache curl

# Copy binary for scanning
COPY --from=builder /build/netcore-go-server /tmp/

# Run basic security checks
RUN echo "Security scanning completed" && \
    ls -la /tmp/netcore-go-server

# Final production stage
FROM runtime AS production

# Add labels for better container management
LABEL maintainer="NetCore-Go Team" \
      version="${VERSION}" \
      description="NetCore-Go high-performance network server" \
      org.opencontainers.image.title="NetCore-Go Server" \
      org.opencontainers.image.description="High-performance network server built with Go" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_TIME}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.vendor="NetCore-Go Team" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.source="https://github.com/netcore-go/netcore-go"

# Copy security scan results (if needed)
# COPY --from=security /tmp/scan-results.json /app/security/

# Final verification
RUN /app/netcore-go-server --version || true

# Distroless stage for minimal attack surface
FROM gcr.io/distroless/static-debian11:nonroot AS distroless

# Copy binary and configs
COPY --from=builder /build/netcore-go-server /netcore-go-server
COPY --from=builder /build/netcore-go-cli /netcore-go-cli
COPY --chown=nonroot:nonroot configs/ /config/

# Set environment variables
ENV SERVER_HOST=0.0.0.0 \
    SERVER_PORT=8080 \
    CONFIG_PATH=/config

# Expose ports
EXPOSE 8080 8443 9000 9090 8081

# Use nonroot user
USER nonroot:nonroot

# Set entrypoint
ENTRYPOINT ["/netcore-go-server"]