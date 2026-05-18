# Echo Proxy With Headers

This project is a reverse proxy built using the Echo framework in Golang. It supports modifying request and response headers, removing unwanted headers, and dynamically configuring upstream services via environment variables.

## Features
- Proxy multiple upstream services based on the `Host` header.
- Modify request and response headers.
- Remove specific headers from responses.
- Rewrite request paths using regex patterns.
- Conditional proxying based on headers or query parameters.
- Configuration via `json` file.
- Supports Docker deployment.

## Installation
### As a Library
You can use this project as a library in your own Go application:

```go
import (
	"github.com/roketid/echo-proxy"
)

func main() {
	configs := echoproxy.LoadConfig()
	port := "8080"
	echoproxy.RunProxy(configs, port)
}
```

### As a Standalone Application
#### Prerequisites
- Install [Go](https://go.dev/)
- Clone this repository:
  ```sh
  git clone https://github.com/roketid/echo-proxy.git
  cd echoproxy
  ```
- Build and run with config file:
  ```sh
  go build -o proxy-server ./cmd/main.go
  ./proxy-server -config config.json
  ```
- Or run with base64-encoded config from environment:
  ```sh
  # Encode your config.json as base64
  export PROXY_CONFIG=$(base64 -w 0 < config.json)
  ./proxy-server -config-env PROXY_CONFIG
  ```

## Configuration

### Via `config.json` File
Create a `config.json` file with the following example configuration:

```
{
  "example.com": {
    "upstream": "https://example.com",
    "host_override": "",
    "request_headers": {"X-Custom-Header": "MyValue"},
    "response_headers": {"X-Response-Header": "ResponseValue"},
    "remove_headers": ["Server", "X-Powered-By", "Set-Cookie"],
    "condition": {
      "header": "X-Api-Key",
      "value": "secret-key"
    },
    "fallback_behavior": "404"
  },
  "another.com": {
    "upstream": "https://another.com",
    "host_override": "",
    "request_headers": {"X-Another-Header": "AnotherValue"},
    "response_headers": {"X-Another-Response": "AnotherResponseValue"},
    "remove_headers": ["Server", "Set-Cookie"]
  }
}
```

### Via Environment Variable (Base64 Encoded)
You can also pass your configuration as a base64-encoded JSON string via an environment variable. This is useful for containerized deployments:

```bash
# Encode your config.json as base64
export PROXY_CONFIG=$(base64 -w 0 < config.json)

# Run the proxy with ENV config
./proxy-server -config-env PROXY_CONFIG
```

**Example with inline config:**
```bash
# Create base64 config
CONFIG_JSON='{"example.com": {"upstream": "https://example.com"}}'
export PROXY_CONFIG=$(echo -n "$CONFIG_JSON" | base64 -w 0)

# Run the proxy
./proxy-server -config-env PROXY_CONFIG
```

This approach is ideal for Kubernetes, Docker, and other containerized environments where you want to pass configuration through environment variables.

### Conditional Proxying
You can add a `condition` object to a host config to only proxy requests that match a specific header or query parameter value. Only equality is supported:

- To match a header:
  ```json
  "condition": {
    "header": "X-Api-Key",
    "value": "secret-key"
  }
  ```
- To match a query parameter:
  ```json
  "condition": {
    "query_param": "token",
    "value": "mytoken"
  }
  ```

If the condition is not met, the proxy will use the `fallback_behavior` (e.g., "404", "bad_gateway", or proxy to a fallback upstream if `fallback_upstream` is set).

### Path Rewriting
You can rewrite request paths using regex patterns by adding `path_rewrite_regex` and `path_rewrite_replacement` to your host config:

```json
{
  "api.example.com": {
    "upstream": "https://backend.example.com",
    "path_rewrite_regex": "^/v1/(.*)",
    "path_rewrite_replacement": "/api/$1"
  }
}
```

This configuration will rewrite requests from `/v1/users` to `/api/users` before forwarding to the upstream server.

Common use cases:
- **Add prefix**: Transform `/users` → `/api/v2/users`
  ```json
  "path_rewrite_regex": "^/(.*)",
  "path_rewrite_replacement": "/api/v2/$1"
  ```

- **Remove prefix**: Transform `/v1/users` → `/users`
  ```json
  "path_rewrite_regex": "^/v1/(.*)",
  "path_rewrite_replacement": "/$1"
  ```

- **Replace path segment**: Transform `/old/path` → `/new/path`
  ```json
  "path_rewrite_regex": "^/old/(.*)",
  "path_rewrite_replacement": "/new/$1"
  ```

### Timeout Configuration
You can configure timeout settings for each upstream service (all values in seconds):

```json
{
  "api.example.com": {
    "upstream": "https://backend.example.com",
    "dial_timeout": 30,      // Connection establishment timeout
    "read_timeout": 30,      // Read operation timeout
    "write_timeout": 30,     // Write operation timeout
    "idle_timeout": 90       // Connection idle timeout
  }
}
```

Default timeouts are:
- `dial_timeout`: 30 seconds
- `read_timeout`: 30 seconds
- `write_timeout`: 30 seconds
- `idle_timeout`: 90 seconds

### Performance Optimizations for High Traffic
The proxy includes several optimizations for handling high-traffic scenarios:

- **Pre-compiled regex patterns**: Path rewriting regex patterns are compiled once at startup, not per-request
- **Pre-parsed URLs**: Upstream URLs are parsed once at initialization
- **Connection pooling**: Configurable HTTP connection pooling with persistent connections
  - Max 100 concurrent connections per host
  - Max 100 idle connections
  - 30-second keep-alive interval
- **Efficient header management**: Fast header modification and removal
- **Concurrent request handling**: Full support for concurrent requests with proper resource management

## Using Docker
### Build and Run the Container
```sh
docker build -t roketid/echo-proxy .
docker run -p 8080:8080 -v $(PWD)/config.json:/app/config.json roketid/echo-proxy
```

### Using Pre-built Image (GitHub Container Registry)
```sh
docker run -p 8080:8080 -v $(PWD)/config.json:/app/config.json ghcr.io/roketid/echo-proxy:main
```

## GitHub Actions - Automated Docker Image Deployment
This project includes a GitHub Actions workflow that builds and pushes the Docker image to GitHub Container Registry when changes are pushed to the `main` branch.

## Contributing
Feel free to submit issues or pull requests for improvements.

## License
MIT License
