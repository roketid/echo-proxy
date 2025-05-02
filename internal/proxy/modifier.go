package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/brotli/go/cbrotli"
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
	for _, header := range config.RemovedHeaders {
		res.Header.Del(header)
	}
	// Add additional response headers
	for key, value := range config.ResponseHeaders {
		res.Header.Set(key, value)
	}
}

// ModifyResponseContent modifies the response content (text based only)
func ModifyResponseContent(res *http.Response, config ProxyConfig) error {
	if config.ContentReplacers == nil && len(config.ContentReplacers) == 0 {
		return nil
	}

	// Only modify text-based content
	contentType := res.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text") && !strings.Contains(contentType, "json") {
		return nil
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error read response body, %v", err)
	}
	res.Body.Close()

	// Decode Brotli if Content-Encoding is "br"
	contentEncoding := res.Header.Get("Content-Encoding")
	var decodedBody []byte
	if contentEncoding == "br" {
		reader := cbrotli.NewReader(bytes.NewReader(bodyBytes))
		decodedBody, err = io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("Error decoding Brotli response: %v", err)
		}
	} else {
		decodedBody = bodyBytes
	}

	modifiedBody := string(decodedBody)
	for oldValue, newValue := range config.ContentReplacers {
		modifiedBody = strings.ReplaceAll(modifiedBody, oldValue, newValue)
	}

	// Replace the response body with the modified content
	res.Body = io.NopCloser(strings.NewReader(modifiedBody))
	res.ContentLength = int64(len(modifiedBody))
	res.Header.Set("Content-Length", fmt.Sprint(len(modifiedBody)))

	return nil
}
