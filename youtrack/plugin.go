package youtrack

import (
	"context"

	"github.com/turbot/steampipe-plugin-sdk/v6/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin/transform"
)

const pluginName = "steampipe-plugin-youtrack"

// Plugin returns the YouTrack Steampipe plugin definition.
func Plugin(ctx context.Context) *plugin.Plugin {
	return &plugin.Plugin{
		Name:             pluginName,
		DefaultTransform: transform.FromCamel(),
		ConnectionConfigSchema: &plugin.ConnectionConfigSchema{
			NewInstance: ConfigInstance,
		},
		TableMap: tableMap(ctx),
	}
}
