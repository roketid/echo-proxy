package proxy

import (
	"encoding/json"
	"log"
	"os"
)

// ProxyConfig holds the configuration for the proxy
type ProxyConfig struct {
	Upstream        string            `json:"upstream"`
	HostOverride    string            `json:"host_override"`
	RequestHeaders  map[string]string `json:"request_headers"`
	ResponseHeaders map[string]string `json:"response_headers"`
	RemoveHeaders   []string          `json:"remove_headers"`

	// Conditional proxying
	Condition *ProxyCondition `json:"condition,omitempty"`
	// Fallback behavior: "fallback_upstream", "404", "bad_gateway", etc.
	FallbackBehavior string `json:"fallback_behavior,omitempty"`
	FallbackUpstream string `json:"fallback_upstream,omitempty"`
}

// ProxyCondition defines a condition for proxying
// Only one of Header or QueryParam is checked per condition
// If Value matches, proxy to Upstream, else fallback
// Example: {"header": "X-Api-Key", "value": "secret"}
type ProxyCondition struct {
	Header     string `json:"header,omitempty"`
	QueryParam string `json:"query_param,omitempty"`
	Value      string `json:"value"`
}

// LoadConfig loads the proxy configuration from a JSON file
func LoadConfig(configFile string) map[string]ProxyConfig {
	configs := make(map[string]ProxyConfig)

	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Failed to read config file %s: %v", configFile, err)
	}

	if err := json.Unmarshal(data, &configs); err != nil {
		log.Fatalf("Failed to parse JSON config: %v", err)
	}

	return configs
}
