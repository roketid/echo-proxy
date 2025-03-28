package proxy

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// ProxyConfig holds the configuration for the proxy
type ProxyConfig struct {
	Upstream        string            `json:"upstream"`
	HostOverride    string            `json:"host_override"`
	RequestHeaders  map[string]string `json:"request_headers"`
	ResponseHeaders map[string]string `json:"response_headers"`
	RemovedHeaders  []string          `json:"removed_headers"`
}

// LoadConfig loads the proxy configuration from a JSON file
func LoadConfig(configFile string) map[string]ProxyConfig {
	configs := make(map[string]ProxyConfig)

	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Failed to read config file %s: %v", configFile, err)
	}

	if err := json.Unmarshal(data, &configs); err != nil {
		log.Fatalf("Failed to parse JSON config: %v", err)
	}

	return configs
}
