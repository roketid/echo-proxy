package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/roketid/echo-proxy/internal/proxy"
	"github.com/stretchr/testify/assert"
)

func TestProxyHandler(t *testing.T) {
	// Create a test upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-Response", "Success")
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
			RemovedHeaders:  []string{"X-Test-Response"},
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
	assert.NotContains(t, rec.Header(), "X-Test-Response") // Ensure removed header is gone
	assert.Equal(t, "ProxyTest", rec.Header().Get("X-Proxy-Header"))
}

