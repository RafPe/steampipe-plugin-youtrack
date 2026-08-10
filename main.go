// The steampipe-plugin-youtrack command serves the YouTrack plugin.
package main

import (
	"github.com/RafPe/steampipe-youtrack/youtrack"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		PluginName: "youtrack",
		PluginFunc: youtrack.Plugin,
	})
}
