# Production Ready Service Example

A comprehensive example demonstrating NetCore-Go's production readiness features including error handling, logging, validation, health checks, monitoring, and performance optimization.

## Features

### 🚀 Production Readiness
- **Comprehensive Error Handling**: Structured error codes, messages, and HTTP status mapping
- **Advanced Logging**: Structured JSON logging with context, fields, and sanitization
- **Configuration Validation**: Robust validation rules with custom validators
- **Health Checks**: Liveness, readiness, and startup probes
- **Metrics Collection**: Performance and business metrics
- **Panic Recovery**: Graceful panic handling and recovery
- **Performance Monitoring**: CPU, memory, and goroutine monitoring
- **Graceful Shutdown**: Clean service termination

### 🔧 Core Components
- **ProductionManager**: Central orchestrator for all production features
- **Logger**: Production-grade structured logging with async support
- **Validator**: Flexible validation system with built-in and custom rules
- **ErrorHandler**: Intelligent error handling with retry logic
- **HealthChecker**: Kubernetes-compatible health endpoints
- **MetricsCollector**: Prometheus-style metrics collection

## Quick Start

### Prerequisites
- Go 1.19 or higher
- NetCore-Go framework

### Installation

1. Navigate to the example directory:
```bash
cd examples/production-ready
```

2. Install dependencies:
```bash
go mod tidy
```

3. Run the service:
```bash
go run main.go
```

4. The service will start with multiple endpoints:
   - **Main API**: `http://localhost:8080`
   - **Health Checks**: `http://localhost:8081/health`
   - **Metrics**: `http://localhost:8082/metrics`

## API Endpoints

### User Management

#### Create User
```http
POST /api/users
Content-Type: application/json

{
  "name": "John Doe",
  "email": "john@example.com",
  "age": 30
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "name": "John Doe",
    "email": "john@example.com",
    "age": 30,
    "created_at": "2024-01-01T12:00:00Z",
    "updated_at": "2024-01-01T12:00:00Z"
  },
  "timestamp": "2024-01-01T12:00:00Z",
  "request_id": "req-123"
}
```

#### Get User
```http
GET /api/users?id=123e4567-e89b-12d3-a456-426614174000
```

#### Update User
```http
PUT /api/users?id=123e4567-e89b-12d3-a456-426614174000
Content-Type: application/json

{
  "name": "Jane Doe",
  "age": 25
}
```

#### Delete User
```http
DELETE /api/users?id=123e4567-e89b-12d3-a456-426614174000
```

#### List Users
```http
GET /api/users
```

### System Endpoints

#### Service Status
```http
GET /api/status
```

**Response:**
```json
{
  "service_name": "user-service",
  "version": "1.0.0",
  "environment": "development",
  "instance_id": "hostname-1234-1640995200",
  "running": true,
  "uptime_seconds": 3600,
  "memory_alloc": 1048576,
  "goroutines": 10,
  "health_check_enabled": true,
  "metrics_enabled": true
}
```

#### Health Checks
```http
GET /health          # Overall health
GET /health/live     # Liveness probe
GET /health/ready    # Readiness probe
```

#### Metrics
```http
GET /metrics
```

## Error Handling

### Error Response Format
```json
{
  "success": false,
  "error": "User not found",
  "code": "NOT_FOUND",
  "timestamp": "2024-01-01T12:00:00Z",
  "request_id": "req-123"
}
```

### Error Codes
- `INVALID_REQUEST`: Bad request data
- `NOT_FOUND`: Resource not found
- `INTERNAL_ERROR`: Server error
- `VALIDATION_ERROR`: Input validation failed
- `UNAUTHORIZED`: Authentication required
- `FORBIDDEN`: Access denied

## Logging

### Log Levels
- `TRACE`: Detailed debugging information
- `DEBUG`: Debug information
- `INFO`: General information
- `WARN`: Warning messages
- `ERROR`: Error conditions
- `FATAL`: Fatal errors (exits application)
- `PANIC`: Panic conditions (panics application)

### Log Format (Development)
```
2024-01-01T12:00:00Z [INFO] [user-service] main.go:123: User created successfully {user_id=123, email=john@example.com}
```

### Log Format (Production)
```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "level": "INFO",
  "message": "User created successfully",
  "service_name": "user-service",
  "request_id": "req-123",
  "fields": {
    "user_id": "123",
    "email": "joh***"
  }
}
```

## Validation

### Built-in Validators
- `Required`: Field must not be empty
- `MinLength`: Minimum string/array length
- `MaxLength`: Maximum string/array length
- `Range`: Numeric range validation
- `Regex`: Regular expression matching
- `URL`: Valid URL format
- `IP`: Valid IP address
- `Port`: Valid port number (1-65535)
- `Duration`: Valid time duration
- `OneOf`: Value must be in allowed list

### Custom Validation Rules
```go
// Add custom validation rule
validator.AddRule("Email", &EmailRule{})
validator.AddRule("Age", NewRangeRule(0, 150))
```

## Configuration

### Production Configuration
```go
config := core.DefaultProductionConfig("my-service")
config.Environment = "production"
config.Logging.Level = core.InfoLevel
config.Logging.Format = core.JSONFormat
config.HealthCheck.Enabled = true
config.Metrics.Enabled = true
```

### Development Configuration
```go
config := core.DefaultProductionConfig("my-service")
config.Environment = "development"
config.Logging = core.DevelopmentLoggerConfig("my-service")
```

## Monitoring

### Health Check Endpoints
- `/health`: Overall service health
- `/health/live`: Liveness probe (Kubernetes)
- `/health/ready`: Readiness probe (Kubernetes)
- `/health/startup`: Startup probe (Kubernetes)

### Metrics
- `memory_usage_bytes`: Current memory usage
- `goroutine_count`: Number of active goroutines
- `http_requests_total`: Total HTTP requests
- `http_request_duration_ms`: Request duration

### Performance Monitoring
- CPU usage monitoring
- Memory usage alerts
- Goroutine leak detection
- GC performance tuning

## Production Deployment

### Docker
```dockerfile
FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080 8081 8082
CMD ["./main"]
```

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: user-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: user-service
  template:
    metadata:
      labels:
        app: user-service
    spec:
      containers:
      - name: user-service
        image: user-service:latest
        ports:
        - containerPort: 8080
        - containerPort: 8081
        - containerPort: 8082
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8081
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 5
        startupProbe:
          httpGet:
            path: /health/startup
            port: 8081
          initialDelaySeconds: 10
          periodSeconds: 10
          failureThreshold: 30
        env:
        - name: ENVIRONMENT
          value: "production"
        - name: LOG_LEVEL
          value: "INFO"
        resources:
          requests:
            memory: "64Mi"
            cpu: "250m"
          limits:
            memory: "128Mi"
            cpu: "500m"
```

### Environment Variables
```bash
# Service Configuration
SERVICE_NAME=user-service
VERSION=1.0.0
ENVIRONMENT=production

# Logging
LOG_LEVEL=INFO
LOG_FORMAT=json
LOG_FILE=/var/log/app.log

# Health Checks
HEALTH_CHECK_PORT=8081
HEALTH_CHECK_ENABLED=true

# Metrics
METRICS_PORT=8082
METRICS_ENABLED=true

# Performance
MAX_MEMORY_USAGE=1073741824  # 1GB
MAX_GOROUTINES=10000
GC_PERCENT=100
```

## Testing

### Manual Testing
1. Start the service: `go run main.go`
2. Test API endpoints using curl or Postman
3. Check health endpoints
4. Monitor logs and metrics

### Example Test Commands
```bash
# Create a user
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"name":"John Doe","email":"john@example.com","age":30}'

# Get service status
curl http://localhost:8080/api/status

# Check health
curl http://localhost:8081/health

# Get metrics
curl http://localhost:8082/metrics
```

### Load Testing
```bash
# Install hey (HTTP load testing tool)
go install github.com/rakyll/hey@latest

# Run load test
hey -n 1000 -c 10 http://localhost:8080/api/users
```

## Best Practices

### Error Handling
- Always use structured error codes
- Include request IDs for tracing
- Log errors with appropriate context
- Return user-friendly error messages
- Implement retry logic for transient errors

### Logging
- Use structured logging in production
- Include request context in logs
- Sanitize sensitive information
- Use appropriate log levels
- Implement log rotation

### Validation
- Validate all input data
- Use custom validators for business rules
- Return detailed validation errors
- Sanitize user input

### Performance
- Monitor key metrics
- Set resource limits
- Implement graceful degradation
- Use connection pooling
- Optimize garbage collection

### Security
- Implement authentication and authorization
- Use HTTPS in production
- Sanitize logs to prevent information leakage
- Implement rate limiting
- Validate and sanitize all inputs

## Troubleshooting

### Common Issues

**Service won't start**
- Check port availability
- Verify configuration
- Check logs for startup errors

**High memory usage**
- Monitor goroutine count
- Check for memory leaks
- Adjust GC settings

**Health checks failing**
- Verify health check endpoints
- Check service dependencies
- Review health check configuration

**Logs not appearing**
- Check log level configuration
- Verify log output destination
- Check file permissions

### Debug Mode
```bash
# Run with debug logging
LOG_LEVEL=DEBUG go run main.go

# Enable profiler
PROFILER_ENABLED=true go run main.go
# Access profiler at http://localhost:6060/debug/pprof/
```

## Extensions

### Possible Enhancements
1. **Database Integration**: Add PostgreSQL/MySQL support
2. **Caching**: Implement Redis caching layer
3. **Message Queues**: Add RabbitMQ/Kafka integration
4. **Distributed Tracing**: Implement OpenTelemetry
5. **Rate Limiting**: Add request rate limiting
6. **Authentication**: Implement JWT authentication
7. **API Versioning**: Support multiple API versions
8. **Circuit Breaker**: Add circuit breaker pattern
9. **Bulk Operations**: Support batch operations
10. **Event Sourcing**: Implement event-driven architecture

## License

This example is part of the NetCore-Go project and follows the same license terms.