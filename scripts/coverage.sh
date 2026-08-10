#!/bin/sh
set -eu

coverage_file="$(mktemp "${TMPDIR:-/tmp}/steampipe-youtrack-coverage.XXXXXX")"
trap 'rm -f "$coverage_file"' EXIT HUP INT TERM

# The executable-only root package contains only plugin.Serve wiring and is the
# explicit coverage-policy allowlist. Every testable first-party package must
# maintain 100% statement coverage.
packages="$(go list ./... | grep -v '^github.com/RafPe/steampipe-youtrack$')"
go test -covermode=atomic -coverprofile="$coverage_file" $packages
go tool cover -func="$coverage_file"

total="$(go tool cover -func="$coverage_file" | awk '/^total:/ {print $3}')"
if [ "$total" != "100.0%" ]; then
	echo "first-party statement coverage is $total; want 100.0%" >&2
	exit 1
fi
