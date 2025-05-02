package proxy

import (
	"fmt"
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

	// Reverse proxy handler
	e.Any("/*", func(c echo.Context) error {
		host := c.Request().Host
		config, exists := pm.Proxies[host]
		if !exists {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "No upstream for host"})
		}

		target, err := url.Parse(config.Upstream)
		if err != nil {
			return err
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ModifyResponse = func(res *http.Response) error {
			ModifyResponseHeaders(res, config)

			if err := ModifyResponseContent(res, config); err != nil {
				fmt.Println("error ModifyResponseContent,", err)
			}

			return nil
		}

		proxy.Director = func(req *http.Request) {
			ModifyRequest(req, c, target, config)
		}

		proxy.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	return e
}
