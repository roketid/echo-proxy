package middleware

import (
	"bytes"
	"fmt"
	"io"
	"log"

	"github.com/labstack/echo/v4"
)

// RequestLoggerMiddleware logs request details (IP, headers, payload, cookies)
func RequestLoggerMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := c.Request()

		// Capture Remote IP Address
		remoteAddr := c.RealIP()

		// Capture Headers
		headers := req.Header

		// Capture Cookies
		var cookies []string
		for _, cookie := range req.Cookies() {
			cookies = append(cookies, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
		}

		// Capture Body (Clone to avoid modifying the original request)
		var requestBody string
		if req.Body != nil {
			bodyBytes, err := io.ReadAll(req.Body)
			if err == nil {
				requestBody = string(bodyBytes)
				// Restore the original body so it can be read later
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// Log the captured data
		log.Printf("Incoming Request:\n"+
			"RemoteAddr: %s\nHeaders: %v\nCookies: %v\nBody: %s\n",
			remoteAddr, headers, cookies, requestBody,
		)

		// Continue with the next middleware/handler
		return next(c)
	}
}
