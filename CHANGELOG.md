# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Per-Route Headers**: a route in `routes` can set `request_headers`, `response_headers` and `remove_headers` for the requests it serves
  - Header maps are merged with the host's header by header; the route's value wins, names compared without case
  - `remove_headers` on a route replaces the host's list (an empty list removes nothing)
  - The other routes and `fallback_behavior`/`fallback_upstream` keep the host's settings; timeouts stay host-level
  - Fully backward compatible: routes without these keys behave as before

### Fixed
- **Path Rewrite to an Empty String**: `path_rewrite_replacement: ""` now strips what `path_rewrite_regex` matched (e.g. `^/api` turns `/api/users` into `/users`), for hosts and routes alike. It used to skip the rewrite. A config that set a regex with an empty replacement and relied on nothing happening now gets the rewrite

### Testing
- Added tests for prefix stripping with an empty replacement, on a host, on a route and from a config file
- Added tests for per-route header merging (including case), `remove_headers` replacement, route headers on a host without any, headers staying on their route without changing the host's config, and loading the new keys from a config file

## [1.2.0] - 2026-09-01

### Added
- **Multiple Upstreams per Host (Routes)**: Route a single host to more than two upstreams via an ordered `routes` list
  - Each route has its own optional `condition` (`header`, `query_param`, or the new `path_prefix` matcher) and `upstream`
  - Evaluated in order; first match wins, and a route with no `condition` acts as a catch-all
  - Per-route overrides for `upstream`, `host_override`, and `path_rewrite_regex`/`path_rewrite_replacement`; shared settings (`request_headers`, `response_headers`, `remove_headers`, timeouts) stay at the host level
  - Falls back to the host's existing `fallback_behavior`/`fallback_upstream` when no route matches
  - Fully backward compatible: existing single `upstream` + `condition` + `fallback_upstream` configs are unaffected

### Changed
- **Configuration Structure**: Added new optional fields:
  - `Routes []Route` on `ProxyConfig`
  - New `Route` struct: `Condition`, `Upstream`, `HostOverride`, `PathRewriteRegex`, `PathRewriteReplacement`
  - `PathPrefix` added to `ProxyCondition` as a third condition type alongside `Header`/`QueryParam`

### Testing
- Added test coverage for routes: header-based matching, path-prefix matching, catch-all fallthrough, no-match fallback behavior, and per-route `host_override`

## [1.1.1] - 2026-08-30

### Security
- **Dependency Upgrade**: Bumped `echo`, `gommon`, `testify`, and `golang.org/x/{crypto,net,sys,text,time}` to their latest patched versions to resolve Dependabot vulnerability alerts
  - `go.mod` now requires Go >= 1.25; `Dockerfile.root` builder image bumped to `golang:1.25` to match
  - `govulncheck` now reports 0 reachable vulnerabilities, down from 28 stdlib + 13 imported-package findings

## [1.1.0] - 2026-08-30

### Added
- **Fallback for Unmatched Hosts**: A reserved `"*"` wildcard config entry is now used when a request's `Host` header matches no configured host
  - Proxies to a default upstream, or serves a static `index.html`, before falling back to the existing `502 Bad Gateway`
  - Configurable via `DEFAULT_UPSTREAM` / `DEFAULT_STATIC_INDEX_FILE` environment variables when `config.json` doesn't define a `"*"` entry

### Security
- **GitHub Actions Hardening**: Restricted the `docker-publish` workflow's push trigger from all branches (`*`) to `main` only, keeping tag-based triggers for releases

## [1.0.0] - 2026-05-18

### Added

#### Core Features
- **Path Rewriting**: Regex-based path rewriting for URL transformation
  - Configure via `path_rewrite_regex` and `path_rewrite_replacement` fields
  - Support for adding, removing, or replacing path segments
  - Examples: `/v1/users` → `/users`, `/old/path` → `/new/path`

- **Base64 Environment Variable Configuration**: Load proxy config from base64-encoded JSON via environment variables
  - New `-config-env` command-line flag
  - Ideal for containerized deployments (Kubernetes, Docker)
  - Backward compatible with file-based configuration

- **Docker PROXY_CONFIG Auto-Detection**: Docker images now intelligently select configuration method
  - If `PROXY_CONFIG` environment variable is set → uses `-config-env`
  - Otherwise → uses config file from `CONFIG_PATH`
  - Works with both `Dockerfile` and `Dockerfile.root`

- **Configurable Timeouts**: Per-upstream timeout settings
  - `dial_timeout`: Connection establishment timeout (default: 30s)
  - `read_timeout`: Read operation timeout (default: 30s)
  - `write_timeout`: Write operation timeout (default: 30s)
  - `idle_timeout`: Connection idle timeout (default: 90s)

#### Performance Optimizations
- **Pre-compiled Regex Patterns**: Regex patterns are compiled once at startup instead of per-request
- **Pre-parsed URLs**: Upstream URLs are parsed once at initialization
- **HTTP Connection Pooling**: Optimized for high-traffic scenarios
  - Maximum 100 concurrent connections per host
  - Maximum 100 idle connections
  - 30-second keep-alive interval
- **Efficient Request Handling**: Optimized for concurrent request processing with proper resource management

#### Documentation
- **Realistic Example Configuration**: Updated `config.example.json` to use httpbin.org
  - All examples are now functional and testable
  - Demonstrates all major features (headers, conditional routing, path rewriting, timeouts)
- **Comprehensive README**: Updated with Docker usage examples and configuration documentation

#### Security
- **GitHub Actions Hardening**: Restricted workflow triggers to main branch and version tags
- **Image Signing**: Docker images are automatically signed with cosign + sigstore for provenance

### Changed
- **Configuration Structure**: Added new optional fields to ProxyConfig:
  - `PathRewriteRegex`, `PathRewriteReplacement`: For path transformation
  - `DialTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`: Timeout settings in seconds
  - `CompiledRegex`, `ParsedURL`, `ParsedFallbackURL`: Internal optimization fields (not JSON-serialized)

### Fixed
- Removed unused parameter from `initializeConfig` function
- Improved error handling for URL parsing and regex compilation

### Infrastructure
- Updated `.github/workflows/docker-publish.yml` to restrict triggers and improve security
- Added comprehensive test coverage for new features (5 passing tests)
- All code passes `go vet` and `go fmt` checks

### Technical Details

#### New Command-Line Options
```bash
# Config from file (traditional)
./proxy-server -config config.json

# Config from base64-encoded environment variable
./proxy-server -config-env PROXY_CONFIG
```

#### Docker Usage
```bash
# Using config file
docker run -p 8080:8080 -v $(PWD)/config.json:/app/config.json roketid/echo-proxy

# Using base64 environment variable
export PROXY_CONFIG=$(base64 -w 0 < config.json)
docker run -p 8080:8080 -e PROXY_CONFIG="$PROXY_CONFIG" roketid/echo-proxy
```

#### Example Configuration
```json
{
  "api.example.com": {
    "upstream": "https://backend.example.com",
    "path_rewrite_regex": "^/v1/(.*)",
    "path_rewrite_replacement": "/$1",
    "request_headers": {"X-Api-Version": "v1"},
    "dial_timeout": 15,
    "read_timeout": 20,
    "idle_timeout": 90
  }
}
```

### Testing
- All 5 tests passing:
  - TestProxyHandler
  - TestProxyConditionHeader
  - TestProxyConditionQueryParam
  - TestProxyRewritePath
  - TestProxyFallbackBehavior
- Verified with httpbin.org integration testing
- Docker image builds and runs successfully

### Known Limitations
- Path rewriting applies to all matching requests (no per-condition rewriting)
- Timeout values are global per upstream (not request-specific)

### Migration Guide

#### From v0.x (if upgrading)
- Existing config files continue to work without changes
- New timeout and path rewriting features are optional
- No breaking changes to existing configuration

### Contributors
- Claude Haiku 4.5 (AI Assistant)

### Links
- [GitHub Repository](https://github.com/roketid/echo-proxy)
- [Docker Image](https://github.com/roketid/echo-proxy/pkgs/container/echo-proxy)
