# iskndr

A lightweight CLI client for creating HTTP tunnels to expose local applications to the internet.

## Installation

```bash
go install github.com/igneel64/iskandar/iskndr/cmd/iskndr@latest
```

Or build from source:

```bash
cd iskndr
go build ./cmd/iskndr
```

## Usage

### Basic Tunnel

Expose a local application running on port 8080:

```bash
iskndr tunnel 8080 --server tunnel.example.com
```

This creates a tunnel and provides you with a public URL like `https://abc123.tunnel.example.com` that forwards to your local `localhost:8080`.

### Custom Host and Port

Tunnel to a specific host and port:

```bash
iskndr tunnel localhost:3000 --server tunnel.example.com
```

### HTTPS with Self-Signed Certificates

If your tunnel server uses self-signed certificates (common for local development):

```bash
iskndr tunnel 8080 --server https://tunnel.localhost.direct --allow-insecure
```

### Enable Logging

For debugging or production monitoring, enable structured logging:

```bash
iskndr tunnel 8080 --server tunnel.example.com --logging
```

## License

MIT License - see [LICENSE](../LICENSE) for details.
