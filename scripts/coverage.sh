#!/bin/sh
set -eu

coverage_file="$(mktemp "${TMPDIR:-/tmp}/steampipe-youtrack-coverage.XXXXXX")"
trap 'rm -f "$coverage_file"' EXIT HUP INT TERM

# Two executable-only packages contain nothing but wiring and are the
# explicit coverage-policy allowlist: the root package (plugin.Serve) and
# cmd/releasectl (release.Run, per task-2-brief.md). Every testable
# first-party package, including internal/release itself, must maintain
# 100% statement coverage.
packages="$(go list ./... | grep -v '^github.com/RafPe/steampipe-youtrack$' | grep -v '^github.com/RafPe/steampipe-youtrack/cmd/releasectl$')"
go test -covermode=atomic -coverprofile="$coverage_file" $packages
go tool cover -func="$coverage_file"

total="$(go tool cover -func="$coverage_file" | awk '/^total:/ {print $3}')"
if [ "$total" != "100.0%" ]; then
	echo "first-party statement coverage is $total; want 100.0%" >&2
	exit 1
fi
