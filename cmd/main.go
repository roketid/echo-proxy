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
	// Define and parse the command-line argument for the config file
	configPath := flag.String("config", "", "Path to the proxy config JSON file")
	flag.Parse()

	if *configPath == "" {
		fmt.Println("Usage: ./echo-proxy -config=config.json")
		os.Exit(1)
	}

	configs := proxy.LoadConfig(*configPath)

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
