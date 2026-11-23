package proxy

import (
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
)

// ModifyRequest modifies the outgoing request
func ModifyRequest(req *http.Request, c echo.Context, target *url.URL, config ProxyConfig) {
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.URL.Path = c.Request().URL.Path

	// Set Host override if specified
	if config.HostOverride != "" {
		req.Host = config.HostOverride
	} else {
		req.Host = target.Host
	}

	// Set request headers
	for key, value := range config.RequestHeaders {
		req.Header.Set(key, value)
	}
	// The client's IP as e.IPExtractor found it (see ClientIPExtractor), so a
	// client cannot pass the upstream an address of its own choosing, through
	// either header.
	clientIP := c.RealIP()
	req.Header.Set("X-Forwarded-For", clientIP)
	req.Header.Set("X-Real-IP", clientIP)

	// Handle path rewriting if specified (using pre-compiled regex). An empty
	// replacement is a valid rewrite: it strips what the regex matched.
	if config.CompiledRegex != nil {
		req.URL.Path = config.CompiledRegex.ReplaceAllString(req.URL.Path, config.PathRewriteReplacement)
	}
}

// ModifyResponseHeaders modifies the response headers
func ModifyResponseHeaders(res *http.Response, config ProxyConfig) {
	// Remove unwanted response headers
	for _, header := range config.RemoveHeaders {
		res.Header.Del(header)
	}
	// Add additional response headers
	for key, value := range config.ResponseHeaders {
		res.Header.Set(key, value)
	}
}
