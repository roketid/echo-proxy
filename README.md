# Echo Proxy With Headers

This project is a reverse proxy built using the Echo framework in Golang. It supports modifying request and response headers, removing unwanted headers, and dynamically configuring upstream services via environment variables.

## Features
- Proxy multiple upstream services based on the `Host` header.
- Modify request and response headers.
- Remove specific headers from responses.
- Rewrite request paths using regex patterns.
- Conditional proxying based on headers or query parameters.
- Route a single host to multiple upstreams via ordered rules (header, query param, or path prefix).
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

### Multiple Upstreams per Host (Routes)
A single host key normally maps to one `upstream` (with an optional `condition` + `fallback_upstream` for a two-way choice). If you need to route the same host to more than two upstreams, use `routes` instead: an ordered list of rules, each with its own `condition` and `upstream`. The first matching route wins; a route with no `condition` is a catch-all and should go last.

```json
{
  "api.example.com": {
    "routes": [
      { "condition": { "header": "target-app", "value": "b" }, "upstream": "https://service-b.example.com" },
      { "condition": { "query_param": "region", "value": "eu" }, "upstream": "https://eu.example.com" },
      { "condition": { "path_prefix": "/files/" }, "upstream": "https://files.example.com" },
      { "upstream": "https://default.example.com" }
    ],
    "request_headers": {},
    "remove_headers": ["Server"]
  }
}
```

- Conditions support `header`, `query_param` (matched by equality against `value`), or `path_prefix` (matches when the request path starts with the given prefix).
- Each route can set its own `upstream`, `host_override`, and `path_rewrite_regex`/`path_rewrite_replacement`.
- A route can also set `request_headers`, `response_headers` and `remove_headers` for the requests it serves. Header maps are merged with the host's header by header, the route's value winning (names compared without case); `remove_headers` replaces the host's list (an empty list removes nothing). The other routes and the fallback keep the host's settings, and timeouts are always the host's. A header the host sets can be given another value on a route, not dropped.

  ```json
  {
    "api.example.com": {
      "request_headers": { "X-Source": "edge" },
      "remove_headers": ["Server"],
      "routes": [
        {
          "condition": { "path_prefix": "/kv/" },
          "upstream": "https://kv.example.com",
          "request_headers": { "Cookie": "session=secret" },
          "remove_headers": ["Server", "Set-Cookie"]
        },
        { "upstream": "https://default.example.com" }
      ]
    }
  }
  ```
- If no route matches, the host's `fallback_behavior`/`fallback_upstream` is used, just like the single-`condition` case (defaulting to `502 Bad Gateway` if unset).
- `routes` takes precedence over the top-level `upstream`/`condition` fields when both are present, but existing configs using only `upstream`/`condition` keep working unchanged.

### Fallback for Unmatched Hosts
If a request's `Host` header doesn't match any configured host, the proxy normally responds with `502 Bad Gateway`. You can override this by adding a reserved `"*"` entry to your config — it's used only when nothing else matches:

```json
{
  "*": {
    "upstream": "https://default-upstream.example.com"
  }
}
```

If the `"*"` entry has an `upstream`, unmatched requests are reverse-proxied to it just like any other host (headers, timeouts, path rewriting, etc. all apply). If it has no `upstream` but sets `static_index_file`, that file is served instead:

```json
{
  "*": {
    "static_index_file": "./public/index.html"
  }
}
```

If neither is set (or no `"*"` entry exists at all), the proxy falls back to the default `502 Bad Gateway` response.

#### Configuring the Fallback via Environment Variables
If your config doesn't define a `"*"` entry, you can set it via environment variables instead — handy when you don't want to edit `config.json` per environment:

- `DEFAULT_UPSTREAM`: default upstream URL for unmatched hosts.
- `DEFAULT_STATIC_INDEX_FILE`: path to a static HTML file to serve for unmatched hosts (used only if `DEFAULT_UPSTREAM` is not set).

```bash
export DEFAULT_UPSTREAM="https://default-upstream.example.com"
./proxy-server -config config.json
```

An explicit `"*"` entry in `config.json` (or `PROXY_CONFIG`) always takes precedence over these environment variables.

### Client IP and Trusted Proxies
The proxy sends the client's IP to the upstream in `X-Forwarded-For` and `X-Real-IP`, and logs it. By default that IP is the connection's own address: whatever `X-Forwarded-For` or `X-Real-IP` a client sends is replaced, so it cannot pass the upstream an address of its choosing.

When echo-proxy runs behind another proxy (Cloudflare, a load balancer, an ingress), the connection's address is that proxy's. List it in `TRUSTED_PROXIES` (IP addresses or CIDR ranges, comma-separated):

```bash
export TRUSTED_PROXIES="173.245.48.0/20, 103.21.244.0/22, 10.0.0.0/8"
./proxy-server -config config.json
```

`X-Forwarded-For` is then read only for connections from a listed address: the client is the first address from the right that is not a trusted proxy. Connections from anywhere else still use their own address. Only the listed ranges are trusted, private and loopback addresses included, so list your load balancer's subnet if it is on one. An invalid entry stops the proxy at startup.

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
docker build -f docker/Dockerfile -t roketid/echo-proxy .
docker run -p 8080:8080 -v $(PWD)/config.json:/app/config.json roketid/echo-proxy
```

### With Base64-Encoded Config (Environment Variable)
The Docker image automatically detects if `PROXY_CONFIG` environment variable is set and uses it:

```sh
# Encode your config to base64
CONFIG_B64=$(base64 -w 0 < config.json)

# Run with base64 config
docker run -p 8080:8080 -e PROXY_CONFIG="$CONFIG_B64" roketid/echo-proxy
```

### Using Pre-built Image (GitHub Container Registry)
```sh
# With config file
docker run -p 8080:8080 -v $(PWD)/config.json:/app/config.json ghcr.io/roketid/echo-proxy:main

# With base64 config from environment
docker run -p 8080:8080 -e PROXY_CONFIG="$CONFIG_B64" ghcr.io/roketid/echo-proxy:main
```

### How It Works
The Dockerfile CMD automatically selects the right approach:
- If `PROXY_CONFIG` environment variable is set → uses `-config-env PROXY_CONFIG`
- Otherwise → uses config file from `CONFIG_PATH` (default: `/app/config.json`)

This makes it flexible for both traditional volume-mounted configs and containerized deployments using environment variables.

## GitHub Actions - Automated Docker Image Deployment
This project includes a GitHub Actions workflow that builds and pushes the Docker image to GitHub Container Registry when changes are pushed to the `main` branch.

## Contributing
Feel free to submit issues or pull requests for improvements.

## License
MIT License
