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

	// Conditional proxying (single upstream + fallback upstream)
	Condition *ProxyCondition `json:"condition,omitempty"`
	// Fallback behavior: "fallback_upstream", "404", "bad_gateway", etc.
	FallbackBehavior string `json:"fallback_behavior,omitempty"`
	FallbackUpstream string `json:"fallback_upstream,omitempty"`
	// Rewrite path if needed
	PathRewriteRegex       string `json:"path_rewrite_regex,omitempty"`       // regex pattern
	PathRewriteReplacement string `json:"path_rewrite_replacement,omitempty"` // replacement string

	// Routes allows routing the same host to multiple upstreams based on
	// per-route conditions. Evaluated in order; the first matching route
	// wins. A route with no Condition is a catch-all. When Routes is
	// non-empty it takes precedence over Upstream/Condition above. If no
	// route matches, FallbackBehavior/FallbackUpstream is used.
	Routes []Route `json:"routes,omitempty"`

	// StaticIndexFile is served directly when this config has no Upstream.
	// Used for the "*" wildcard entry to serve a static page for unmatched hosts.
	StaticIndexFile string `json:"static_index_file,omitempty"`

	// Pre-compiled regex for performance (not JSON serialized)
	CompiledRegex *regexp.Regexp `json:"-"`
	// Pre-parsed URL for performance (not JSON serialized)
	ParsedURL         *url.URL `json:"-"`
	ParsedFallbackURL *url.URL `json:"-"`
	// Timeout settings in seconds
	DialTimeout  int `json:"dial_timeout,omitempty"`
	ReadTimeout  int `json:"read_timeout,omitempty"`
	WriteTimeout int `json:"write_timeout,omitempty"`
	IdleTimeout  int `json:"idle_timeout,omitempty"`
}

// ProxyCondition defines a condition for proxying.
// Only one of Header, QueryParam, or PathPrefix is checked per condition.
// Header/QueryParam match when the request value equals Value.
// PathPrefix matches when the request path starts with PathPrefix (Value is unused).
// Example: {"header": "X-Api-Key", "value": "secret"}
// Example: {"path_prefix": "/api/"}
type ProxyCondition struct {
	Header     string `json:"header,omitempty"`
	QueryParam string `json:"query_param,omitempty"`
	PathPrefix string `json:"path_prefix,omitempty"`
	Value      string `json:"value,omitempty"`
}

// Route is a single rule within ProxyConfig.Routes: an optional Condition
// paired with the upstream (and overrides) to use when it matches.
type Route struct {
	Condition *ProxyCondition `json:"condition,omitempty"`

	Upstream               string `json:"upstream"`
	HostOverride           string `json:"host_override,omitempty"`
	PathRewriteRegex       string `json:"path_rewrite_regex,omitempty"`
	PathRewriteReplacement string `json:"path_rewrite_replacement,omitempty"`

	// RequestHeaders and ResponseHeaders are added to the host's for the
	// requests this route serves, header by header: the route's value wins
	// (names compared without case). RemoveHeaders, when set, replaces the
	// host's list for this route; an empty list removes nothing.
	RequestHeaders  map[string]string `json:"request_headers,omitempty"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	RemoveHeaders   []string          `json:"remove_headers,omitempty"`

	// Pre-parsed/pre-compiled, not JSON serialized.
	ParsedURL     *url.URL       `json:"-"`
	CompiledRegex *regexp.Regexp `json:"-"`
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

		// Pre-parse/compile each route
		for i := range cfg.Routes {
			route := &cfg.Routes[i]
			if route.PathRewriteRegex != "" {
				re, err := regexp.Compile(route.PathRewriteRegex)
				if err != nil {
					log.Fatalf("Failed to compile route regex for host %s: %v", host, err)
				}
				route.CompiledRegex = re
			}
			if route.Upstream != "" {
				parsedURL, err := url.Parse(route.Upstream)
				if err != nil {
					log.Fatalf("Failed to parse route upstream URL for host %s: %v", host, err)
				}
				route.ParsedURL = parsedURL
			}
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

	applyEnvFallback(configs)
	return configs
}

// applyEnvFallback populates the "*" wildcard entry from DEFAULT_UPSTREAM /
// DEFAULT_STATIC_INDEX_FILE environment variables when the config doesn't
// already define one. An explicit "*" entry in config.json / PROXY_CONFIG
// always takes precedence.
func applyEnvFallback(configs map[string]ProxyConfig) {
	if _, exists := configs[WildcardHost]; exists {
		return
	}

	upstream := os.Getenv("DEFAULT_UPSTREAM")
	staticIndexFile := os.Getenv("DEFAULT_STATIC_INDEX_FILE")
	if upstream == "" && staticIndexFile == "" {
		return
	}

	cfg := ProxyConfig{Upstream: upstream, StaticIndexFile: staticIndexFile}
	initializeConfig(&cfg)
	configs[WildcardHost] = cfg
}
