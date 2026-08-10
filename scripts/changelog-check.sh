#!/bin/sh
set -eu

# Pinned Changie version, run via `go run` so nothing is installed globally.
# Bump this together with the matching comment in .changie.yaml.
changie_version="v1.25.2"

# Fixed version used to exercise `batch --dry-run`; never actually released
# by this script.
test_version="v0.1.0"

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
real_config="$repo_root/.changie.yaml"
fixture_dir="$repo_root/scripts/testdata/changelog-check/fixture"
golden_file="$repo_root/scripts/testdata/changelog-check/batch-dry-run.golden"
fragments_dir="$repo_root/.changes/unreleased"

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/steampipe-youtrack-changelog-check.XXXXXX")"
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM

# changesDir in .changie.yaml is relative; run everything from repo root so
# it (and the fixture override below) resolve regardless of invocation cwd.
cd "$repo_root"

# --- 1. Template/config drift check -----------------------------------
#
# `changie batch` always includes the live .changes/unreleased/ directory,
# so a golden diffed against real fragments would break on every ordinary
# `changie new` a contributor runs. Instead, render a frozen fixture
# (scripts/testdata/changelog-check/fixture/.changes/unreleased/) through
# the REAL .changie.yaml, with only changesDir swapped to point at the
# fixture. Every rendering setting (versionFormat, kindFormat, changeFormat,
# footerFormat, kinds order, newlines) still comes from the real config, so
# this catches genuine drift while staying independent of live content.
fixture_config="$work_dir/fixture.changie.yaml"
sed "s#^changesDir:.*#changesDir: $fixture_dir/.changes#" "$real_config" >"$fixture_config"

raw_file="$work_dir/fixture-raw.txt"
actual_file="$work_dir/actual.txt"

if ! CHANGIE_CONFIG_PATH="$fixture_config" \
	go run "github.com/miniscruff/changie@${changie_version}" batch "$test_version" --dry-run \
	>"$raw_file"; then
	echo "changie failed to render the fixture fragments:" >&2
	cat "$raw_file" >&2
	exit 1
fi

# The rendered version heading embeds today's date, so it can never match a
# checked-in golden file byte-for-byte across days. Normalize any ISO date
# to a fixed placeholder before comparing.
sed 's/[0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}/YYYY-MM-DD/' "$raw_file" >"$actual_file"

if ! diff -u "$golden_file" "$actual_file"; then
	echo "changie batch --dry-run output drifted from $golden_file" >&2
	exit 1
fi

# --- 2. Live fragment validation ----------------------------------------
#
# Separately, validate every real unreleased fragment. Running the real
# config's `batch --dry-run` against .changes/unreleased/ already parses
# each fragment as YAML and rejects any kind not declared in .changie.yaml
# (changie exits non-zero before printing anything on either failure), so
# this doubles as that check. It does not enforce a non-empty body, so
# check that explicitly here.
if ! CHANGIE_CONFIG_PATH="$real_config" \
	go run "github.com/miniscruff/changie@${changie_version}" batch "$test_version" --dry-run \
	>"$work_dir/live.txt"; then
	echo "changie failed to parse .changes/unreleased/ fragments:" >&2
	cat "$work_dir/live.txt" >&2
	exit 1
fi

fragment_count=0
for fragment in "$fragments_dir"/*.yaml; do
	[ -e "$fragment" ] || continue
	fragment_count=$((fragment_count + 1))

	body="$(sed -n 's/^body: *//p' "$fragment" | head -n 1)"
	case "$body" in
	'' | '""' | "''")
		echo "fragment $fragment has an empty body" >&2
		exit 1
		;;
	esac
done

echo "changelog-check: ok ($fragment_count unreleased fragment(s), changie ${changie_version})"
