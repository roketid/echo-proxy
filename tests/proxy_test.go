package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/roketid/echo-proxy/internal/proxy"
	"github.com/stretchr/testify/assert"
)

const testResponseHeader = "X-Test-Response"
const condComHost = "cond.com"
const apiKeyHeader = "X-Api-Key"

func TestProxyHandler(t *testing.T) {
	// Create a test upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(testResponseHeader, "Success")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from upstream"))
	}))
	defer upstream.Close()

	// Define proxy configuration
	config := map[string]proxy.ProxyConfig{
		"example.com": {
			Upstream:        upstream.URL,
			HostOverride:    "",
			RequestHeaders:  map[string]string{"X-Custom-Header": "TestValue"},
			ResponseHeaders: map[string]string{"X-Proxy-Header": "ProxyTest"},
			RemoveHeaders:   []string{testResponseHeader},
		},
	}

	// Initialize proxy
	proxyManager := proxy.NewProxyManager(config)
	e := proxyManager.NewProxy() // This should return an Echo instance

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "example.com"

	rec := httptest.NewRecorder()

	// Handle request using Echo
	e.ServeHTTP(rec, req)

	// Validate response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Hello from upstream")
	assert.NotContains(t, rec.Header(), testResponseHeader) // Ensure removed header is gone
	assert.Equal(t, "ProxyTest", rec.Header().Get("X-Proxy-Header"))
}

func TestProxyConditionHeader(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Condition met"))
	}))
	defer upstream.Close()

	config := map[string]proxy.ProxyConfig{
		condComHost: {
			Upstream: upstream.URL,
			Condition: &proxy.ProxyCondition{
				Header: apiKeyHeader,
				Value:  "secret-key",
			},
			FallbackBehavior: "404",
		},
	}
	proxyManager := proxy.NewProxyManager(config)
	e := proxyManager.NewProxy()

	// Should proxy (header matches)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = condComHost
	req.Header.Set(apiKeyHeader, "secret-key")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Condition met")

	// Should fallback (header does not match)
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Host = condComHost
	req2.Header.Set(apiKeyHeader, "wrong-key")
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusNotFound, rec2.Code)
}

func TestProxyConditionQueryParam(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Query param met"))
	}))
	defer upstream.Close()

	config := map[string]proxy.ProxyConfig{
		condComHost: {
			Upstream: upstream.URL,
			Condition: &proxy.ProxyCondition{
				QueryParam: "token",
				Value:      "abc123",
			},
			FallbackBehavior: "404",
		},
	}
	proxyManager := proxy.NewProxyManager(config)
	e := proxyManager.NewProxy()

	// Should proxy (query param matches)
	req := httptest.NewRequest(http.MethodGet, "/?token=abc123", nil)
	req.Host = condComHost
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Query param met")

	// Should fallback (query param does not match)
	req2 := httptest.NewRequest(http.MethodGet, "/?token=wrong", nil)
	req2.Host = condComHost
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusNotFound, rec2.Code)
}

func TestProxyRewritePath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back the path for verification
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Path: " + r.URL.Path))
	}))
	defer upstream.Close()

	config := map[string]proxy.ProxyConfig{
		"rewrite.com": {
			Upstream:               upstream.URL,
			PathRewriteRegex:       "^/old/(.*)",
			PathRewriteReplacement: "/new/$1",
		},
	}
	proxyManager := proxy.NewProxyManager(config)
	e := proxyManager.NewProxy()

	// Should rewrite /old/test to /new/test
	req := httptest.NewRequest(http.MethodGet, "/old/test", nil)
	req.Host = "rewrite.com"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Path: /new/test")

	// Should rewrite /old/abc/123 to /new/abc/123
	req2 := httptest.NewRequest(http.MethodGet, "/old/abc/123", nil)
	req2.Host = "rewrite.com"
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "Path: /new/abc/123")
}

func TestProxyFallbackBehavior(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Fallback upstream"))
	}))
	defer upstream.Close()

	config := map[string]proxy.ProxyConfig{
		"fallback.com": {
			Upstream:         upstream.URL,
			FallbackBehavior: "fallback_upstream",
			FallbackUpstream: upstream.URL,
		},
	}
	proxyManager := proxy.NewProxyManager(config)
	e := proxyManager.NewProxy()

	// Should fallback to the configured upstream
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "fallback.com"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Fallback upstream")
}
