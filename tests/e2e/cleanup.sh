#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
export E2E_UID=${E2E_UID:-$(id -u)}
export E2E_GID=${E2E_GID:-$(id -g)}

docker compose --project-directory "$repo_dir" -f "$repo_dir/docker-compose.yml" \
  down --volumes --remove-orphans

state_dir="$repo_dir/tests/e2e/.state"
if [ -d "$state_dir" ]; then
  find "$state_dir" -depth -mindepth 1 -delete
fi

printf '%s\n' "e2e: containers, volumes, and generated state removed"
