package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/roketid/echo-proxy/internal/middleware"
	"github.com/roketid/echo-proxy/internal/proxy"
)

func main() {
	configPath := flag.String("config", "", "Path to the proxy config JSON file")
	envVar := flag.String("config-env", "", "Environment variable containing base64-encoded proxy config JSON")
	flag.Parse()

	var configs map[string]proxy.ProxyConfig

	// Load config from environment variable (base64 encoded) if specified
	if *envVar != "" {
		configs = proxy.LoadConfigFromEnv(*envVar)
	} else if *configPath != "" {
		// Load config from file
		configs = proxy.LoadConfig(*configPath)
	} else {
		fmt.Println("Usage: ./echo-proxy -config=config.json")
		fmt.Println("   or: ./echo-proxy -config-env=PROXY_CONFIG")
		fmt.Println("\nWhere PROXY_CONFIG is a base64-encoded JSON configuration")
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	proxyServer := proxy.NewProxyManager(configs)
	e := proxyServer.NewProxy()

	// Register the logging middleware
	e.Use(middleware.RequestLoggerMiddleware)

	log.Printf("Starting proxy server on port %s...\n", port)
	e.Start(":" + port)
}
