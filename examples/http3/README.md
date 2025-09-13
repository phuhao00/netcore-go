# HTTP/3 Server Example

This example demonstrates how to create an HTTP/3 server using the NetCore-Go framework.

## Features

- ✅ **HTTP/3 Support** - Full HTTP/3 protocol implementation using quic-go
- 🔒 **TLS Required** - HTTP/3 requires TLS certificates
- 📊 **Statistics** - Built-in connection and request statistics
- 🏥 **Health Checks** - Health monitoring endpoint
- ⚡ **High Performance** - QUIC protocol for improved performance

## Prerequisites

HTTP/3 requires TLS certificates. You can generate self-signed certificates for testing:

```bash
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes
```

## Running the Example

1. Generate TLS certificates (see Prerequisites)
2. Build and run the server:

```bash
cd examples/http3
go build -o http3-server main.go
./http3-server
```

## Testing the Server

Since HTTP/3 is relatively new, you'll need a client that supports HTTP/3:

### Using curl (if HTTP/3 support is compiled in):
```bash
curl --http3 -k https://localhost:8443/
curl --http3 -k https://localhost:8443/health
```

### Using Chrome/Chromium:
1. Open Chrome with HTTP/3 enabled: `chrome --enable-quic --quic-version=h3`
2. Navigate to `https://localhost:8443/`
3. Accept the self-signed certificate warning

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Simple greeting message |
| GET | `/health` | Health check endpoint |

## Configuration

The server can be configured by modifying the `HTTP3Config` in `main.go`:

```go
config := http3.DefaultHTTP3Config()
config.Host = "localhost"
config.Port = 8443
config.CertFile = "server.crt"
config.KeyFile = "server.key"
config.MaxIdleTimeout = 300 * time.Second
config.KeepAlivePeriod = 30 * time.Second
```

## Notes

- HTTP/3 uses UDP instead of TCP
- TLS 1.3 is required for HTTP/3
- Some firewalls may block UDP traffic on port 8443
- Browser support for HTTP/3 is still evolving

## Troubleshooting

### Certificate Issues
If you get certificate errors, make sure:
1. The certificate files exist in the current directory
2. The certificate is valid and not expired
3. The certificate includes the correct hostname/IP

### Connection Issues
If clients can't connect:
1. Check that UDP port 8443 is not blocked by firewall
2. Verify the server is listening on the correct interface
3. Ensure the client supports HTTP/3

### Performance
For production use:
1. Use proper TLS certificates from a trusted CA
2. Configure appropriate timeouts and limits
3. Monitor connection statistics using `server.GetStats()`