#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)

docker compose --project-directory "$repo_dir" -f "$repo_dir/docker-compose.yml" config --quiet
sh -n "$repo_dir/tests/e2e/run.sh"
sh -n "$repo_dir/tests/e2e/cleanup.sh"

grep -F 'jetbrains/youtrack:2026.1.13874' "$repo_dir/docker-compose.yml" >/dev/null
grep -F 'steampipe_version=2.3.2' "$repo_dir/tests/e2e/run.sh" >/dev/null
grep -F 'checksums.txt' "$repo_dir/tests/e2e/run.sh" >/dev/null
grep -F 'sha256' "$repo_dir/tests/e2e/run.sh" >/dev/null
grep -F 'chmod 600' "$repo_dir/tests/e2e/run.sh" >/dev/null
grep -F 'base_url = "http://127.0.0.1:' "$repo_dir/tests/e2e/run.sh" >/dev/null
grep -F '/api/admin/timeTrackingSettings/workItemTypes' "$repo_dir/tests/e2e/run.sh" >/dev/null
grep -F '/timeTrackingSettings?fields=' "$repo_dir/tests/e2e/run.sh" >/dev/null
if grep -F 'work-item creation rejected' "$repo_dir/tests/e2e/run.sh" >/dev/null; then
  printf '%s\n' "e2e: work-item coverage must not be skipped" >&2
  exit 1
fi
if grep -F 'token_file=' "$repo_dir/tests/e2e/run.sh" >/dev/null; then
  printf '%s\n' "e2e: token must not be duplicated into a standalone file" >&2
  exit 1
fi
if grep -E 'image: .+:(latest|main|master)$' "$repo_dir/docker-compose.yml" >/dev/null; then
  printf '%s\n' "e2e: container images must use immutable version tags" >&2
  exit 1
fi

printf '%s\n' "e2e: static harness checks passed"
