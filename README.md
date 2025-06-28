# Echo Proxy With Headers

This project is a reverse proxy built using the Echo framework in Golang. It supports modifying request and response headers, removing unwanted headers, and dynamically configuring upstream services via environment variables.

## Features
- Proxy multiple upstream services based on the `Host` header.
- Modify request and response headers.
- Remove specific headers from responses.
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
- Build and run:
  ```sh
  go build -o proxy-server
  ./proxy-server -config config.json
  ```

## Configuration via `config.json`
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
