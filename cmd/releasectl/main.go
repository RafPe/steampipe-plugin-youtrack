// Command releasectl validates PR release metadata and computes the next
// release version for steampipe-youtrack's review-gated release process.
// See internal/release for the CLI contract and all logic; this file is
// intentionally a thin wrapper with none of its own.
package main

import (
	"os"

	"github.com/RafPe/steampipe-youtrack/internal/release"
)

func main() {
	os.Exit(release.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
