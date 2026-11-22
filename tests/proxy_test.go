package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestNoMatchWithoutWildcard(t *testing.T) {
	config := map[string]proxy.ProxyConfig{
		"known.com": {Upstream: "http://localhost:9999"},
	}
	proxyManager := proxy.NewProxyManager(config)
	e := proxyManager.NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "unknown.com"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestNoMatchFallsBackToWildcardUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Default upstream"))
	}))
	defer upstream.Close()

	config := map[string]proxy.ProxyConfig{
		"known.com":        {Upstream: "http://localhost:9999"},
		proxy.WildcardHost: {Upstream: upstream.URL},
	}
	proxyManager := proxy.NewProxyManager(config)
	e := proxyManager.NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "unknown.com"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Default upstream")
}

func TestProxyRoutesHeaderMatch(t *testing.T) {
	upstreamA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("From A"))
	}))
	defer upstreamA.Close()

	upstreamB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("From B"))
	}))
	defer upstreamB.Close()

	upstreamDefault := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("From default"))
	}))
	defer upstreamDefault.Close()

	config := map[string]proxy.ProxyConfig{
		"multi.com": {
			Routes: []proxy.Route{
				{Condition: &proxy.ProxyCondition{Header: "target-app", Value: "a"}, Upstream: upstreamA.URL},
				{Condition: &proxy.ProxyCondition{Header: "target-app", Value: "b"}, Upstream: upstreamB.URL},
				{Upstream: upstreamDefault.URL},
			},
		},
	}
	proxyManager := proxy.NewProxyManager(config)
	e := proxyManager.NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "multi.com"
	req.Header.Set("target-app", "a")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "From A")

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Host = "multi.com"
	req2.Header.Set("target-app", "b")
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "From B")

	// No matching header -> falls through to the catch-all (no-condition) route
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.Host = "multi.com"
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusOK, rec3.Code)
	assert.Contains(t, rec3.Body.String(), "From default")
}

func TestProxyRoutesPathPrefixMatch(t *testing.T) {
	upstreamAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("API service"))
	}))
	defer upstreamAPI.Close()

	upstreamFiles := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Files service"))
	}))
	defer upstreamFiles.Close()

	config := map[string]proxy.ProxyConfig{
		"path-route.com": {
			Routes: []proxy.Route{
				{Condition: &proxy.ProxyCondition{PathPrefix: "/api/"}, Upstream: upstreamAPI.URL},
				{Condition: &proxy.ProxyCondition{PathPrefix: "/files/"}, Upstream: upstreamFiles.URL},
			},
		},
	}
	proxyManager := proxy.NewProxyManager(config)
	e := proxyManager.NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Host = "path-route.com"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "API service")

	req2 := httptest.NewRequest(http.MethodGet, "/files/report.pdf", nil)
	req2.Host = "path-route.com"
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "Files service")
}

func TestProxyRoutesNoMatchUsesFallbackBehavior(t *testing.T) {
	upstreamA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("From A"))
	}))
	defer upstreamA.Close()

	config := map[string]proxy.ProxyConfig{
		"no-match.com": {
			Routes: []proxy.Route{
				{Condition: &proxy.ProxyCondition{Header: "target-app", Value: "a"}, Upstream: upstreamA.URL},
			},
			FallbackBehavior: "404",
		},
	}
	proxyManager := proxy.NewProxyManager(config)
	e := proxyManager.NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "no-match.com"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestProxyRoutesHostOverridePerRoute(t *testing.T) {
	var seenHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHost = r.Host
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	config := map[string]proxy.ProxyConfig{
		"override.com": {
			Routes: []proxy.Route{
				{Upstream: upstream.URL, HostOverride: "custom-host.example.com"},
			},
		},
	}
	proxyManager := proxy.NewProxyManager(config)
	e := proxyManager.NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "override.com"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "custom-host.example.com", seenHost)
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadConfigEnvFallbackAppliedWhenMissing(t *testing.T) {
	t.Setenv("DEFAULT_UPSTREAM", "https://default.example.com")
	t.Setenv("DEFAULT_STATIC_INDEX_FILE", "")

	path := writeConfigFile(t, `{"known.com": {"upstream": "https://known.example.com"}}`)
	configs := proxy.LoadConfig(path)

	wildcard, exists := configs[proxy.WildcardHost]
	assert.True(t, exists)
	assert.Equal(t, "https://default.example.com", wildcard.Upstream)
}

func TestLoadConfigEnvFallbackSkippedWhenWildcardConfigured(t *testing.T) {
	t.Setenv("DEFAULT_UPSTREAM", "https://should-not-be-used.example.com")

	path := writeConfigFile(t, `{"*": {"static_index_file": "./index.html"}}`)
	configs := proxy.LoadConfig(path)

	wildcard, exists := configs[proxy.WildcardHost]
	assert.True(t, exists)
	assert.Equal(t, "./index.html", wildcard.StaticIndexFile)
	assert.Equal(t, "", wildcard.Upstream)
}

func TestLoadConfigEnvFallbackNotAppliedWhenEnvUnset(t *testing.T) {
	t.Setenv("DEFAULT_UPSTREAM", "")
	t.Setenv("DEFAULT_STATIC_INDEX_FILE", "")

	path := writeConfigFile(t, `{"known.com": {"upstream": "https://known.example.com"}}`)
	configs := proxy.LoadConfig(path)

	_, exists := configs[proxy.WildcardHost]
	assert.False(t, exists)
}

func TestNoMatchFallsBackToWildcardStaticIndex(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	assert.NoError(t, os.WriteFile(indexPath, []byte("<h1>Default page</h1>"), 0o644))

	config := map[string]proxy.ProxyConfig{
		proxy.WildcardHost: {StaticIndexFile: indexPath},
	}
	proxyManager := proxy.NewProxyManager(config)
	e := proxyManager.NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "unknown.com"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Default page")
}

// echoHeaders answers with the request headers it got as response headers
// prefixed "Seen-", and sets Server and X-Powered-By for remove_headers.
func echoHeaders() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for name, values := range r.Header {
			w.Header().Set("Seen-"+name, values[0])
		}
		w.Header().Set("Server", "nginx")
		w.Header().Set("X-Powered-By", "php")
		w.WriteHeader(http.StatusOK)
	}))
}

func routeHeadersConfig(kv, other string) map[string]proxy.ProxyConfig {
	return map[string]proxy.ProxyConfig{
		"api.com": {
			RequestHeaders:  map[string]string{"X-Host": "host", "X-Shared": "from-host", "Cookie": "host-cookie"},
			ResponseHeaders: map[string]string{"X-Host-Response": "host"},
			RemoveHeaders:   []string{"Server"},
			Routes: []proxy.Route{
				{
					Condition:       &proxy.ProxyCondition{PathPrefix: "/kv/"},
					Upstream:        kv,
					RequestHeaders:  map[string]string{"X-Shared": "from-route", "cookie": "route-cookie", "Authorization": "Bearer route"},
					ResponseHeaders: map[string]string{"X-Route-Response": "route"},
					RemoveHeaders:   []string{"X-Powered-By"},
				},
				{Upstream: other},
			},
		},
	}
}

func serveAPI(e http.Handler, path string, header http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "api.com"
	for name, values := range header {
		req.Header[name] = values
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestProxyRoutesHeadersOnTopOfTheHost(t *testing.T) {
	kv, other := echoHeaders(), echoHeaders()
	defer kv.Close()
	defer other.Close()
	e := proxy.NewProxyManager(routeHeadersConfig(kv.URL, other.URL)).NewProxy()

	rec := serveAPI(e, "/kv/x", http.Header{"Authorization": {"Bearer client"}})
	assert.Equal(t, http.StatusOK, rec.Code)
	got := rec.Header()
	// Request headers: the host's, with the route's on top, names compared without case.
	assert.Equal(t, "host", got.Get("Seen-X-Host"))
	assert.Equal(t, "from-route", got.Get("Seen-X-Shared"))
	assert.Equal(t, "route-cookie", got.Get("Seen-Cookie"))
	assert.Equal(t, "Bearer route", got.Get("Seen-Authorization"))
	// Response headers merged; the route's remove_headers replaces the host's.
	assert.Equal(t, "host", got.Get("X-Host-Response"))
	assert.Equal(t, "route", got.Get("X-Route-Response"))
	assert.Empty(t, got.Get("X-Powered-By"))
	assert.Equal(t, "nginx", got.Get("Server"))
}

func TestProxyRoutesHeadersStayOnTheirRoute(t *testing.T) {
	kv, other := echoHeaders(), echoHeaders()
	defer kv.Close()
	defer other.Close()
	config := routeHeadersConfig(kv.URL, other.URL)
	e := proxy.NewProxyManager(config).NewProxy()

	serveAPI(e, "/kv/x", nil) // the route must not change the host's settings for later requests
	rec := serveAPI(e, "/other", http.Header{"Authorization": {"Bearer client"}})
	got := rec.Header()
	assert.Equal(t, "from-host", got.Get("Seen-X-Shared"))
	assert.Equal(t, "host-cookie", got.Get("Seen-Cookie"))
	assert.Equal(t, "Bearer client", got.Get("Seen-Authorization"))
	assert.Empty(t, got.Get("Server"))
	assert.Equal(t, "php", got.Get("X-Powered-By"))
	assert.Empty(t, got.Get("X-Route-Response"))
	assert.Equal(t, map[string]string{"X-Host": "host", "X-Shared": "from-host", "Cookie": "host-cookie"},
		config["api.com"].RequestHeaders)
}

func TestProxyRoutesHeadersWithoutHostHeaders(t *testing.T) {
	kv := echoHeaders()
	defer kv.Close()
	config := map[string]proxy.ProxyConfig{
		"api.com": {Routes: []proxy.Route{
			{Upstream: kv.URL, RequestHeaders: map[string]string{"X-Only-Route": "yes"}, RemoveHeaders: []string{}},
		}},
	}
	rec := serveAPI(proxy.NewProxyManager(config).NewProxy(), "/", nil)
	assert.Equal(t, "yes", rec.Header().Get("Seen-X-Only-Route"))
	assert.Equal(t, "nginx", rec.Header().Get("Server"))
}

func TestLoadConfigReadsRouteHeaders(t *testing.T) {
	path := writeConfigFile(t, `{"api.com": {"routes": [{"upstream": "http://kv.local",
		"request_headers": {"Cookie": "c"}, "response_headers": {"X-R": "r"}, "remove_headers": ["Set-Cookie"]}]}}`)
	route := proxy.LoadConfig(path)["api.com"].Routes[0]
	assert.Equal(t, map[string]string{"Cookie": "c"}, route.RequestHeaders)
	assert.Equal(t, map[string]string{"X-R": "r"}, route.ResponseHeaders)
	assert.Equal(t, []string{"Set-Cookie"}, route.RemoveHeaders)
}

// pathEcho answers with the path it was asked for.
func pathEcho() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Path: " + r.URL.Path))
	}))
}

func getPath(e http.Handler, host, path string) string {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = host
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Body.String()
}

func TestProxyRewritePathWithEmptyReplacementStripsTheMatch(t *testing.T) {
	upstream := pathEcho()
	defer upstream.Close()
	config := map[string]proxy.ProxyConfig{
		"strip.com": {Upstream: upstream.URL, PathRewriteRegex: "^/api", PathRewriteReplacement: ""},
		"route-strip.com": {Routes: []proxy.Route{
			{Condition: &proxy.ProxyCondition{PathPrefix: "/kv/"}, Upstream: upstream.URL, PathRewriteRegex: "^/kv"},
		}},
	}
	e := proxy.NewProxyManager(config).NewProxy()

	assert.Equal(t, "Path: /users/1", getPath(e, "strip.com", "/api/users/1"))
	assert.Equal(t, "Path: /other", getPath(e, "strip.com", "/other")) // no match: unchanged
	assert.Equal(t, "Path: /namespaces", getPath(e, "route-strip.com", "/kv/namespaces"))
}

func TestLoadConfigEmptyReplacementStripsTheMatch(t *testing.T) {
	upstream := pathEcho()
	defer upstream.Close()
	path := writeConfigFile(t, `{"strip.com": {"upstream": "`+upstream.URL+`",
		"path_rewrite_regex": "^/api", "path_rewrite_replacement": ""}}`)
	e := proxy.NewProxyManager(proxy.LoadConfig(path)).NewProxy()
	assert.Equal(t, "Path: /users", getPath(e, "strip.com", "/api/users"))
}
