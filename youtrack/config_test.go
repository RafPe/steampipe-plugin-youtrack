package youtrack

import (
	"strings"
	"testing"
)

func TestConfigInstance(t *testing.T) {
	t.Parallel()

	got := ConfigInstance()
	if got == nil {
		t.Fatal("ConfigInstance() = nil, want *Config")
	}
	if _, ok := got.(*Config); !ok {
		t.Errorf("ConfigInstance() type = %T, want *Config", got)
	}
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		baseURL string
		token   string
		wantErr string
	}{
		"hosted instance":        {baseURL: "https://example.youtrack.cloud", token: "test-token"},
		"subpath instance":       {baseURL: "https://example.com/youtrack", token: "token"},
		"localhost HTTP":         {baseURL: "http://localhost:8080/youtrack", token: "token"},
		"loopback IPv4 HTTP":     {baseURL: "http://127.0.0.1:8080", token: "token"},
		"loopback IPv6 HTTP":     {baseURL: "http://[::1]:8080", token: "token"},
		"missing URL":            {token: "token", wantErr: "base_url must be set"},
		"relative URL":           {baseURL: "/youtrack", token: "token", wantErr: "absolute URL"},
		"non-HTTPS remote URL":   {baseURL: "http://example.com", token: "token", wantErr: "HTTPS"},
		"unsupported URL scheme": {baseURL: "ftp://example.com", token: "token", wantErr: "HTTPS"},
		"URL query":              {baseURL: "https://example.com?x=1", token: "token", wantErr: "query"},
		"URL fragment":           {baseURL: "https://example.com/#x", token: "token", wantErr: "fragment"},
		"URL credentials":        {baseURL: "https://user@example.com", token: "token", wantErr: "credentials"},
		"missing token":          {baseURL: "https://example.com", wantErr: "token must be set"},
		"blank token":            {baseURL: "https://example.com", token: "  ", wantErr: "token must be set"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			config := &Config{BaseURL: &tt.baseURL, Token: &tt.token}
			err := ValidateConfig(config)
			if tt.wantErr == "" && err != nil {
				t.Errorf("ValidateConfig(%q, token) error = %v, want nil", tt.baseURL, err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("ValidateConfig(%q, token) error = %v, want error containing %q", tt.baseURL, err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfigDoesNotExposeInvalidBaseURL(t *testing.T) {
	t.Parallel()

	privateValue := strings.Join([]string{"configuration", "private", "value"}, "-")
	baseURL, token := "https://user:"+privateValue+"%zz@example.test", "token"
	err := ValidateConfig(&Config{BaseURL: &baseURL, Token: &token})
	if err == nil {
		t.Fatal("ValidateConfig(invalid credential URL) error = nil, want error")
	}
	if strings.Contains(err.Error(), privateValue) {
		t.Errorf("ValidateConfig(invalid credential URL) error = %q, must not contain URL credentials", err)
	}
}

func TestConfigEnvFallback(t *testing.T) {
	t.Setenv("YOUTRACK_URL", "https://env.youtrack.cloud")
	t.Setenv("YOUTRACK_TOKEN", "env-token")

	resolved := Config{}.withEnvFallback()
	if resolved.BaseURL == nil || *resolved.BaseURL != "https://env.youtrack.cloud" {
		t.Errorf("withEnvFallback() BaseURL = %v, want YOUTRACK_URL value", resolved.BaseURL)
	}
	if resolved.Token == nil || *resolved.Token != "env-token" {
		t.Errorf("withEnvFallback() Token = %v, want YOUTRACK_TOKEN value", resolved.Token)
	}
	if err := ValidateConfig(&resolved); err != nil {
		t.Errorf("ValidateConfig(env fallback) error = %v, want nil", err)
	}
}

func TestConfigEnvFallbackPrefersExplicitConfig(t *testing.T) {
	t.Setenv("YOUTRACK_URL", "https://env.youtrack.cloud")
	t.Setenv("YOUTRACK_TOKEN", "env-token")

	baseURL := "https://explicit.example.com"
	token := strings.Join([]string{"from", "config"}, "-")
	resolved := Config{BaseURL: &baseURL, Token: &token}.withEnvFallback()
	if resolved.BaseURL == nil || *resolved.BaseURL != baseURL {
		t.Errorf("withEnvFallback() BaseURL = %v, want explicit config value %q", resolved.BaseURL, baseURL)
	}
	if resolved.Token == nil || *resolved.Token != token {
		t.Errorf("withEnvFallback() Token = %v, want explicit config value %q", resolved.Token, token)
	}
}

func TestConfigEnvFallbackIgnoresBlankEnvironment(t *testing.T) {
	t.Setenv("YOUTRACK_URL", "  ")
	t.Setenv("YOUTRACK_TOKEN", "")

	resolved := Config{}.withEnvFallback()
	if resolved.BaseURL != nil {
		t.Errorf("withEnvFallback() BaseURL = %q, want nil for blank YOUTRACK_URL", *resolved.BaseURL)
	}
	if resolved.Token != nil {
		t.Errorf("withEnvFallback() Token = %q, want nil for blank YOUTRACK_TOKEN", *resolved.Token)
	}
}

func TestValidateConfigNil(t *testing.T) {
	t.Parallel()

	if err := ValidateConfig(nil); err == nil {
		t.Error("ValidateConfig(nil) error = nil, want an error")
	}
}
