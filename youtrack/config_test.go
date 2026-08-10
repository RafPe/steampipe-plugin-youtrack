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
		"missing URL":            {token: "token", wantErr: "base_url is required"},
		"relative URL":           {baseURL: "/youtrack", token: "token", wantErr: "absolute URL"},
		"non-HTTPS remote URL":   {baseURL: "http://example.com", token: "token", wantErr: "HTTPS"},
		"unsupported URL scheme": {baseURL: "ftp://example.com", token: "token", wantErr: "HTTPS"},
		"URL query":              {baseURL: "https://example.com?x=1", token: "token", wantErr: "query"},
		"URL fragment":           {baseURL: "https://example.com/#x", token: "token", wantErr: "fragment"},
		"URL credentials":        {baseURL: "https://user@example.com", token: "token", wantErr: "credentials"},
		"missing token":          {baseURL: "https://example.com", wantErr: "token is required"},
		"blank token":            {baseURL: "https://example.com", token: "  ", wantErr: "token is required"},
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

func TestValidateConfigNil(t *testing.T) {
	t.Parallel()

	if err := ValidateConfig(nil); err == nil {
		t.Error("ValidateConfig(nil) error = nil, want an error")
	}
}
