package proxy

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"regexp"
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
	// Rewrite path if needed
	PathRewriteRegex       string `json:"path_rewrite_regex,omitempty"`       // regex pattern
	PathRewriteReplacement string `json:"path_rewrite_replacement,omitempty"` // replacement string

	// Pre-compiled regex for performance (not JSON serialized)
	CompiledRegex *regexp.Regexp `json:"-"`
	// Pre-parsed URL for performance (not JSON serialized)
	ParsedURL         *url.URL `json:"-"`
	ParsedFallbackURL *url.URL `json:"-"`
	// Timeout settings in seconds
	DialTimeout   int `json:"dial_timeout,omitempty"`
	ReadTimeout   int `json:"read_timeout,omitempty"`
	WriteTimeout  int `json:"write_timeout,omitempty"`
	IdleTimeout   int `json:"idle_timeout,omitempty"`
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

	return initializeConfigs(configs)
}

// LoadConfigFromEnv loads proxy configuration from a base64-encoded environment variable
func LoadConfigFromEnv(envVar string) map[string]ProxyConfig {
	configStr := os.Getenv(envVar)
	if configStr == "" {
		log.Fatalf("Environment variable %s is not set", envVar)
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(configStr)
	if err != nil {
		log.Fatalf("Failed to decode base64 config from env var %s: %v", envVar, err)
	}

	configs := make(map[string]ProxyConfig)
	if err := json.Unmarshal(data, &configs); err != nil {
		log.Fatalf("Failed to parse JSON config from env var %s: %v", envVar, err)
	}

	return initializeConfigs(configs)
}

// initializeConfigs compiles regexes and parses URLs for all configs
func initializeConfigs(configs map[string]ProxyConfig) map[string]ProxyConfig {
	for host, cfg := range configs {
		// Pre-compile regex if specified
		if cfg.PathRewriteRegex != "" {
			re, err := regexp.Compile(cfg.PathRewriteRegex)
			if err != nil {
				log.Fatalf("Failed to compile regex for host %s: %v", host, err)
			}
			cfg.CompiledRegex = re
		}

		// Pre-parse upstream URL
		if cfg.Upstream != "" {
			parsedURL, err := url.Parse(cfg.Upstream)
			if err != nil {
				log.Fatalf("Failed to parse upstream URL for host %s: %v", host, err)
			}
			cfg.ParsedURL = parsedURL
		}

		// Pre-parse fallback upstream URL
		if cfg.FallbackUpstream != "" {
			parsedURL, err := url.Parse(cfg.FallbackUpstream)
			if err != nil {
				log.Fatalf("Failed to parse fallback upstream URL for host %s: %v", host, err)
			}
			cfg.ParsedFallbackURL = parsedURL
		}

		// Set default timeouts if not specified (in seconds)
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

		configs[host] = cfg
	}
	return configs
}
