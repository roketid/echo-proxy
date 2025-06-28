package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"

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

	if config.Condition != nil && !pm.checkCondition(c, config.Condition) {
		return pm.handleFallback(c, config)
	}

	return pm.serveProxy(c, config.Upstream, config)
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
		if config.FallbackUpstream != "" {
			return pm.serveProxy(c, config.FallbackUpstream, config)
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

func (pm *ProxyManager) serveProxy(c echo.Context, upstream string, config ProxyConfig) error {
	target, err := url.Parse(upstream)
	if err != nil {
		return err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
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
