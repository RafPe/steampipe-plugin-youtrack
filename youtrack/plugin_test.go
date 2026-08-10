package youtrack

import (
	"context"
	"testing"
)

func TestPlugin(t *testing.T) {
	t.Parallel()

	p := Plugin(context.Background())
	if p == nil {
		t.Fatal("Plugin(context.Background()) = nil, want a plugin")
	}
	if p.Name != "steampipe-plugin-youtrack" {
		t.Errorf("Plugin(context.Background()).Name = %q, want %q", p.Name, "steampipe-plugin-youtrack")
	}
	if p.ConnectionConfigSchema == nil {
		t.Error("Plugin(context.Background()).ConnectionConfigSchema = nil, want a schema")
		return
	}
	config, ok := p.ConnectionConfigSchema.NewInstance().(*Config)
	if !ok {
		t.Fatalf("Plugin config instance type = %T, want *Config", p.ConnectionConfigSchema.NewInstance())
	}
	if config == nil {
		t.Error("Plugin config instance = nil, want *Config")
	}
}
