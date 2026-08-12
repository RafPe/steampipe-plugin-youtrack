// The steampipe-plugin-youtrack command serves the YouTrack plugin.
package main

import (
	"fmt"
	"os"

	"github.com/RafPe/steampipe-plugin-youtrack/youtrack"
	"github.com/turbot/steampipe-plugin-sdk/v6/plugin"
)

// version, commit, and date are injected at build time via `-ldflags -X`
// (see .goreleaser.yml). The defaults below apply to `go build`/`go run`
// invocations that don't pass ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		_, _ = fmt.Fprintf(os.Stdout, "steampipe-plugin-youtrack %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	plugin.Serve(&plugin.ServeOpts{
		PluginName: "youtrack",
		PluginFunc: youtrack.Plugin,
	})
}
