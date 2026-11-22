package proxy

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// ProxyManager manages multiple proxies based on request Host
type ProxyManager struct {
	Proxies map[string]ProxyConfig
}

// NewProxyManager initializes a proxy manager
func NewProxyManager(configs map[string]ProxyConfig) *ProxyManager {
	return &ProxyManager{Proxies: configs}
}

// NewProxy creates a new Echo instance with proxy setup
func (pm *ProxyManager) NewProxy() *echo.Echo {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Any("/*", pm.proxyHandler)
	return e
}

// WildcardHost is the reserved config key used as a fallback when the
// request Host does not match any configured host.
const WildcardHost = "*"

func (pm *ProxyManager) proxyHandler(c echo.Context) error {
	host := c.Request().Host
	config, exists := pm.Proxies[host]
	if !exists {
		return pm.handleNoMatch(c)
	}

	// Ensure config is initialized (for tests and backward compatibility)
	if needsInit(config) {
		initializeConfig(&config)
		pm.Proxies[host] = config
	}

	if len(config.Routes) > 0 {
		if route, matched := pm.matchRoute(c, config.Routes); matched {
			return pm.serveProxy(c, effectiveConfig(config, route))
		}
		return pm.handleFallback(c, config)
	}

	if config.Condition != nil && !pm.checkCondition(c, config.Condition) {
		return pm.handleFallback(c, config)
	}

	return pm.serveProxy(c, config)
}

// matchRoute returns the first route whose condition matches the request.
// A route with no condition is a catch-all and always matches.
func (pm *ProxyManager) matchRoute(c echo.Context, routes []Route) (Route, bool) {
	for _, route := range routes {
		if route.Condition == nil || pm.checkCondition(c, route.Condition) {
			return route, true
		}
	}
	return Route{}, false
}

// effectiveConfig applies a matched route's overrides on top of the host's
// shared config. Header maps are merged (the route's value wins);
// remove_headers is replaced when the route sets it. Timeouts stay from the
// host level.
func effectiveConfig(config ProxyConfig, route Route) ProxyConfig {
	effective := config
	effective.Upstream = route.Upstream
	effective.ParsedURL = route.ParsedURL
	if route.HostOverride != "" {
		effective.HostOverride = route.HostOverride
	}
	if route.CompiledRegex != nil {
		effective.PathRewriteRegex = route.PathRewriteRegex
		effective.PathRewriteReplacement = route.PathRewriteReplacement
		effective.CompiledRegex = route.CompiledRegex
	}
	effective.RequestHeaders = mergeHeaders(config.RequestHeaders, route.RequestHeaders)
	effective.ResponseHeaders = mergeHeaders(config.ResponseHeaders, route.ResponseHeaders)
	if route.RemoveHeaders != nil {
		effective.RemoveHeaders = route.RemoveHeaders
	}
	return effective
}

// mergeHeaders returns the host's headers with the route's on top. Names are
// compared in canonical form, so a route's "cookie" replaces the host's
// "Cookie" instead of both being set. The host's map is never modified.
func mergeHeaders(host, route map[string]string) map[string]string {
	if len(route) == 0 {
		return host
	}
	if len(host) == 0 {
		return route
	}
	overridden := make(map[string]bool, len(route))
	for name := range route {
		overridden[http.CanonicalHeaderKey(name)] = true
	}
	merged := make(map[string]string, len(host)+len(route))
	for name, value := range host {
		if !overridden[http.CanonicalHeaderKey(name)] {
			merged[name] = value
		}
	}
	for name, value := range route {
		merged[name] = value
	}
	return merged
}

// handleNoMatch is used when the request Host matches no configured host.
// It falls back to the "*" wildcard config if one is set: proxying to its
// upstream (if configured) or serving its static_index_file, in that order.
func (pm *ProxyManager) handleNoMatch(c echo.Context) error {
	config, exists := pm.Proxies[WildcardHost]
	if !exists {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "No upstream for host"})
	}

	if needsInit(config) {
		initializeConfig(&config)
		pm.Proxies[WildcardHost] = config
	}

	if config.ParsedURL != nil {
		return pm.serveProxy(c, config)
	}

	if config.StaticIndexFile != "" {
		return c.File(config.StaticIndexFile)
	}

	return c.JSON(http.StatusBadGateway, map[string]string{"error": "No upstream for host"})
}

// needsInit reports whether config (or one of its routes) still has
// unparsed upstream URLs, e.g. when built directly in tests rather than
// via LoadConfig.
func needsInit(config ProxyConfig) bool {
	if config.ParsedURL == nil && config.Upstream != "" {
		return true
	}
	for _, route := range config.Routes {
		if route.ParsedURL == nil && route.Upstream != "" {
			return true
		}
	}
	return false
}

func (pm *ProxyManager) checkCondition(c echo.Context, cond *ProxyCondition) bool {
	if cond.PathPrefix != "" {
		return strings.HasPrefix(c.Request().URL.Path, cond.PathPrefix)
	}

	var actual string
	if cond.Header != "" {
		actual = c.Request().Header.Get(cond.Header)
	} else if cond.QueryParam != "" {
		actual = c.QueryParam(cond.QueryParam)
	}
	return actual == cond.Value
}

func (pm *ProxyManager) handleFallback(c echo.Context, config ProxyConfig) error {
	switch config.FallbackBehavior {
	case "fallback_upstream":
		if config.ParsedFallbackURL != nil {
			return pm.serveFallbackProxy(c, config)
		}
		if config.FallbackUpstream != "" {
			// Parse on demand for backward compatibility
			parsedURL, _ := parseURL(config.FallbackUpstream)
			config.ParsedFallbackURL = parsedURL
			return pm.serveFallbackProxy(c, config)
		}
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "No fallback upstream configured"})
	case "404":
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Not found"})
	case "bad_gateway":
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "Bad gateway"})
	default:
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Forbidden"})
	}
}

func (pm *ProxyManager) serveFallbackProxy(c echo.Context, config ProxyConfig) error {
	target := config.ParsedFallbackURL
	proxy := httputil.NewSingleHostReverseProxy(target)

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(config.DialTimeout) * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     time.Duration(config.IdleTimeout) * time.Second,
	}
	proxy.Transport = transport

	proxy.ModifyResponse = func(res *http.Response) error {
		ModifyResponseHeaders(res, config)
		return nil
	}
	proxy.Director = func(req *http.Request) {
		ModifyRequest(req, c, target, config)
	}
	proxy.ServeHTTP(c.Response(), c.Request())
	return nil
}

func (pm *ProxyManager) serveProxy(c echo.Context, config ProxyConfig) error {
	target := config.ParsedURL
	if target == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Invalid upstream URL"})
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Configure HTTP transport with connection pooling and timeouts
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(config.DialTimeout) * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     time.Duration(config.IdleTimeout) * time.Second,
	}
	proxy.Transport = transport

	proxy.ModifyResponse = func(res *http.Response) error {
		ModifyResponseHeaders(res, config)
		return nil
	}
	proxy.Director = func(req *http.Request) {
		ModifyRequest(req, c, target, config)
	}
	proxy.ServeHTTP(c.Response(), c.Request())
	return nil
}

// parseURL is a helper function to parse URLs
func parseURL(urlStr string) (*url.URL, error) {
	return url.Parse(urlStr)
}

// initializeConfig initializes a single config with compiled regex and parsed URLs
func initializeConfig(cfg *ProxyConfig) {
	if cfg.PathRewriteRegex != "" {
		re, _ := regexp.Compile(cfg.PathRewriteRegex)
		cfg.CompiledRegex = re
	}

	if cfg.Upstream != "" {
		parsedURL, _ := parseURL(cfg.Upstream)
		cfg.ParsedURL = parsedURL
	}

	if cfg.FallbackUpstream != "" {
		parsedURL, _ := parseURL(cfg.FallbackUpstream)
		cfg.ParsedFallbackURL = parsedURL
	}

	for i := range cfg.Routes {
		route := &cfg.Routes[i]
		if route.PathRewriteRegex != "" {
			re, _ := regexp.Compile(route.PathRewriteRegex)
			route.CompiledRegex = re
		}
		if route.Upstream != "" {
			parsedURL, _ := parseURL(route.Upstream)
			route.ParsedURL = parsedURL
		}
	}

	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 30
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 30
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 90
	}
}
