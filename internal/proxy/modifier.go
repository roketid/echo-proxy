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
	req.Header.Set("X-Forwarded-For", c.RealIP())
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
