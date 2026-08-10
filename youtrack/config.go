package youtrack

import (
	"errors"
	"net"
	"net/url"
	"strings"
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

// ValidateConfig validates a connection without contacting the YouTrack server.
func ValidateConfig(config *Config) error {
	if config == nil {
		return errors.New("config is required")
	}
	if config.BaseURL == nil || strings.TrimSpace(*config.BaseURL) == "" {
		return errors.New("base_url is required")
	}
	if config.Token == nil || strings.TrimSpace(*config.Token) == "" {
		return errors.New("token is required")
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
