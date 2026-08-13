package youtrack

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
)

// Environment variables consulted when the connection config leaves the
// corresponding argument unset.
const (
	baseURLEnvVar = "YOUTRACK_URL"
	tokenEnvVar   = "YOUTRACK_TOKEN"
)

// Config defines the configuration for a YouTrack connection.
type Config struct {
	BaseURL *string `hcl:"base_url"`
	Token   *string `hcl:"token"`
}

// ConfigInstance returns a new YouTrack connection configuration.
func ConfigInstance() any {
	return &Config{}
}

// withEnvFallback fills unset connection arguments from the YOUTRACK_URL and
// YOUTRACK_TOKEN environment variables. Explicit connection config wins.
func (c Config) withEnvFallback() Config {
	if !isSet(c.BaseURL) {
		if value := os.Getenv(baseURLEnvVar); strings.TrimSpace(value) != "" {
			c.BaseURL = &value
		}
	}
	if !isSet(c.Token) {
		if value := os.Getenv(tokenEnvVar); strings.TrimSpace(value) != "" {
			c.Token = &value
		}
	}
	return c
}

func isSet(value *string) bool {
	return value != nil && strings.TrimSpace(*value) != ""
}

// ValidateConfig validates a connection without contacting the YouTrack server.
func ValidateConfig(config *Config) error {
	if config == nil {
		return errors.New("config is required")
	}
	if !isSet(config.BaseURL) {
		return errors.New("base_url must be set in the connection config or the YOUTRACK_URL environment variable")
	}
	if !isSet(config.Token) {
		return errors.New("token must be set in the connection config or the YOUTRACK_TOKEN environment variable")
	}

	baseURL := strings.TrimSpace(*config.BaseURL)
	parsed, err := url.Parse(baseURL)
	if err != nil || !parsed.IsAbs() || parsed.Hostname() == "" {
		return errors.New("base_url must be a valid absolute URL")
	}
	if parsed.User != nil {
		return errors.New("base_url must not contain credentials")
	}
	if parsed.RawQuery != "" {
		return errors.New("base_url must not contain a query")
	}
	if parsed.Fragment != "" {
		return errors.New("base_url must not contain a fragment")
	}
	if parsed.Scheme != "https" && (parsed.Scheme != "http" || !isLoopbackHost(parsed.Hostname())) {
		return errors.New("base_url must use HTTPS; HTTP is allowed only for localhost or a loopback address")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
