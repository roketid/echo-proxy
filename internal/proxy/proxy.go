package proxy

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
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

func (pm *ProxyManager) proxyHandler(c echo.Context) error {
	host := c.Request().Host
	config, exists := pm.Proxies[host]
	if !exists {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "No upstream for host"})
	}

	// Ensure config is initialized (for tests and backward compatibility)
	if config.ParsedURL == nil && config.Upstream != "" {
		initializeConfig(&config)
		pm.Proxies[host] = config
	}

	if config.Condition != nil && !pm.checkCondition(c, config.Condition) {
		return pm.handleFallback(c, config)
	}

	return pm.serveProxy(c, config)
}

func (pm *ProxyManager) checkCondition(c echo.Context, cond *ProxyCondition) bool {
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
