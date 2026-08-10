#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)

docker compose --project-directory "$repo_dir" -f "$repo_dir/docker-compose.yml" config --quiet
sh -n "$repo_dir/tests/e2e/run.sh"
sh -n "$repo_dir/tests/e2e/cleanup.sh"

grep -F 'jetbrains/youtrack:2026.1.13874' "$repo_dir/docker-compose.yml" >/dev/null
grep -F 'turbot/steampipe:0.22.0' "$repo_dir/docker-compose.yml" >/dev/null
grep -F 'chmod 600' "$repo_dir/tests/e2e/run.sh" >/dev/null
grep -F 'base_url = "http://youtrack:8080"' "$repo_dir/tests/e2e/run.sh" >/dev/null
if grep -F 'token_file=' "$repo_dir/tests/e2e/run.sh" >/dev/null; then
  printf '%s\n' "e2e: token must not be duplicated into a standalone file" >&2
  exit 1
fi
if grep -E 'image: .+:(latest|main|master)$' "$repo_dir/docker-compose.yml" >/dev/null; then
  printf '%s\n' "e2e: container images must use immutable version tags" >&2
  exit 1
fi

printf '%s\n' "e2e: static harness checks passed"
