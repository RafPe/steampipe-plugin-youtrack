#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
state_dir="$repo_dir/tests/e2e/.state"
home_dir="$state_dir/home"
config_dir="$home_dir/.steampipe/config"
plugin_dir="$home_dir/.steampipe/plugins/local/youtrack"
base_url=${YOUTRACK_URL:-http://localhost:${YOUTRACK_PORT:-18080}}
steampipe_version=2.3.2
steampipe_binary="$state_dir/bin/steampipe"
run_suffix="$(date +%H%M%S)$$"
project_short_name="SPET$run_suffix"
project_name="Steampipe E2E $run_suffix"
seeded_issue_summary="Steampipe E2E seeded issue $run_suffix"

compose() {
  docker compose --project-directory "$repo_dir" -f "$repo_dir/docker-compose.yml" "$@"
}

fail() {
  printf '%s\n' "e2e: $*" >&2
  exit 1
}

api() {
  method=$1
  path=$2
  data=${3:-}
  if [ -n "$data" ]; then
    curl --fail --silent --show-error -X "$method" \
      -H "Authorization: Bearer $YOUTRACK_TOKEN" \
      -H 'Accept: application/json' -H 'Content-Type: application/json' \
      --data "$data" "$base_url$path"
  else
    curl --fail --silent --show-error -X "$method" \
      -H "Authorization: Bearer $YOUTRACK_TOKEN" \
      -H 'Accept: application/json' "$base_url$path"
  fi
}

sql_json() {
  "$steampipe_binary" --install-dir "$home_dir/.steampipe" query --output json "$1"
}

assert_count() {
  label=$1
  query=$2
  minimum=${3:-1}
  got=$(sql_json "$query" | jq -er '.[0].result')
  [ "$got" -ge "$minimum" ] || fail "$label returned $got rows, want at least $minimum"
}

for command_name in docker curl jq go; do
  command -v "$command_name" >/dev/null 2>&1 || fail "required command not found: $command_name"
done
[ -n "${YOUTRACK_TOKEN:-}" ] || fail "YOUTRACK_TOKEN is required; see docs/e2e.md"

install_steampipe() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case $(uname -m) in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) fail "unsupported Steampipe architecture: $(uname -m)" ;;
  esac
  case $os in
    darwin) archive="steampipe_darwin_${arch}.zip" ;;
    linux) archive="steampipe_linux_${arch}.tar.gz" ;;
    *) fail "unsupported Steampipe operating system: $os" ;;
  esac

  cache_dir="$state_dir/cache/steampipe/$steampipe_version"
  archive_path="$cache_dir/$archive"
  checksums_path="$cache_dir/checksums.txt"
  mkdir -p "$cache_dir" "$(dirname "$steampipe_binary")"
  if [ ! -f "$archive_path" ]; then
    curl --fail --location --silent --show-error \
      --output "$archive_path" \
      "https://github.com/turbot/steampipe/releases/download/v${steampipe_version}/${archive}"
  fi
  curl --fail --location --silent --show-error \
    --output "$checksums_path" \
    "https://github.com/turbot/steampipe/releases/download/v${steampipe_version}/checksums.txt"

  expected="$cache_dir/${archive}.sha256"
  grep "  ${archive}$" "$checksums_path" >"$expected" || fail "checksum missing for $archive"
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$cache_dir" && sha256sum --check "$(basename "$expected")")
  else
    command -v shasum >/dev/null 2>&1 || fail "required command not found: sha256sum or shasum"
    (cd "$cache_dir" && shasum -a 256 --check "$(basename "$expected")")
  fi

  extract_dir=$(mktemp -d "$state_dir/steampipe.XXXXXX")
  if [ "$os" = darwin ]; then
    command -v unzip >/dev/null 2>&1 || fail "required command not found: unzip"
    unzip -q "$archive_path" -d "$extract_dir"
  else
    tar -xzf "$archive_path" -C "$extract_dir"
  fi
  mv "$extract_dir/steampipe" "$steampipe_binary"
  rmdir "$extract_dir"
  chmod 700 "$steampipe_binary"
  got_version=$($steampipe_binary --version)
  case $got_version in
    *"v${steampipe_version}"*) ;;
    *) fail "Steampipe version check failed" ;;
  esac
}

mkdir -p "$config_dir" "$plugin_dir" "$state_dir"
chmod 700 "$state_dir" "$home_dir" "$home_dir/.steampipe" "$config_dir" "$plugin_dir"
install_steampipe
plugin_binary="$plugin_dir/youtrack.plugin"
(cd "$repo_dir" && go build -o "$plugin_binary" .)
chmod 700 "$plugin_binary"

umask 077
cat >"$config_dir/youtrack.spc" <<EOF
connection "youtrack" {
  plugin   = "local/youtrack"
  base_url = "http://127.0.0.1:${YOUTRACK_PORT:-18080}"
  token    = "${YOUTRACK_TOKEN}"
}

connection "youtrack_secondary" {
  plugin   = "local/youtrack"
  base_url = "http://127.0.0.1:${YOUTRACK_PORT:-18080}"
  token    = "${YOUTRACK_TOKEN}"
}

connection "youtrack_invalid" {
  plugin   = "local/youtrack"
  base_url = "http://127.0.0.1:${YOUTRACK_PORT:-18080}"
  token    = "e2e-intentionally-invalid"
}

connection "youtrack_unavailable" {
  plugin   = "local/youtrack"
  base_url = "http://127.0.0.1:9"
  token    = "e2e-not-a-secret"
}
EOF
chmod 600 "$config_dir/youtrack.spc"

export E2E_UID=${E2E_UID:-$(id -u)}
export E2E_GID=${E2E_GID:-$(id -g)}

cleanup() {
	"$steampipe_binary" --install-dir "$home_dir/.steampipe" service stop --force >/dev/null 2>&1 || true
  for resource in \
    "${agile_id:+/api/agiles/$agile_id}" \
    "${saved_query_id:+/api/savedQueries/$saved_query_id}" \
    "${tag_id:+/api/tags/$tag_id}" \
    "${project_id:+/api/admin/projects/$project_id}"; do
    [ -n "$resource" ] && api DELETE "$resource" >/dev/null 2>&1 || true
  done
  if [ -n "${work_item_type_id:-}" ]; then
    api DELETE "/api/admin/timeTrackingSettings/workItemTypes/$work_item_type_id" >/dev/null 2>&1 || true
  fi
  compose down --remove-orphans >/dev/null 2>&1 || true
  if [ -n "$(compose ps -q 2>/dev/null)" ]; then
    printf '%s\n' "e2e: compose containers remain after cleanup" >&2
    cleanup_failed=1
  fi
  rm -f "$config_dir/youtrack.spc" "$plugin_binary"
  [ "${cleanup_failed:-0}" -eq 0 ]
}
trap cleanup EXIT HUP INT TERM

compose up -d --wait youtrack
current_user=$(api GET '/api/users/me?fields=id,login') ||
  fail "YouTrack is not configured or the permanent token is invalid"
leader_id=$(printf '%s' "$current_user" | jq -er '.id')

project_response=$(api POST '/api/admin/projects?fields=id,name,shortName' "$(jq -n \
  --arg name "$project_name" --arg shortName "$project_short_name" \
  --arg leaderID "$leader_id" \
  '{name: $name, shortName: $shortName, leader: {id: $leaderID}}')")
project_id=$(printf '%s' "$project_response" | jq -er '.id')

work_item_type_response=$(api POST \
  '/api/admin/timeTrackingSettings/workItemTypes?fields=id,name,autoAttached' \
  "$(jq -n --arg name "E2E work item type $run_suffix" '{name: $name, autoAttached: true}')")
work_item_type_id=$(printf '%s' "$work_item_type_response" | jq -er '.id')
api POST "/api/admin/projects/$project_id/timeTrackingSettings?fields=enabled,workItemTypes(id,name,autoAttached)" \
  '{"enabled":true}' >/dev/null

# Seed 101 issues so the list path must cross the client's 100-row page boundary.
issue_number=1
first_issue_id=
first_issue_readable=
while [ "$issue_number" -le 101 ]; do
  issue_response=$(api POST '/api/issues?fields=id,idReadable,summary' "$(jq -n \
    --arg project_id "$project_id" --arg summary "$seeded_issue_summary $issue_number" \
    --argjson number "$issue_number" \
    '{project: {id: $project_id}, summary: $summary} + (if $number == 1 then {description: null} else {} end)')")
  if [ "$issue_number" -eq 1 ]; then
    first_issue_id=$(printf '%s' "$issue_response" | jq -er '.id')
    first_issue_readable=$(printf '%s' "$issue_response" | jq -er '.idReadable')
  fi
  issue_number=$((issue_number + 1))
done

comment_id=$(api POST "/api/issues/$first_issue_id/comments?fields=id,text" \
  "$(jq -n --arg text "E2E comment $run_suffix" '{text: $text}')" | jq -er '.id')

tag_response=$(api POST '/api/tags?fields=id,name' \
  "$(jq -n --arg name "E2E tag $run_suffix" '{name: $name}')")
tag_id=$(printf '%s' "$tag_response" | jq -er '.id')
api POST "/api/issues/$first_issue_id/tags?fields=id" "$(jq -n --arg id "$tag_id" '{id: $id}')" >/dev/null

saved_query_response=$(api POST '/api/savedQueries?fields=id,name,query' "$(jq -n \
  --arg name "E2E saved query $run_suffix" --arg query "project: $project_short_name" \
  '{name: $name, query: $query}')")
saved_query_id=$(printf '%s' "$saved_query_response" | jq -er '.id')

article_response=$(api POST '/api/articles?fields=id,idReadable,summary' "$(jq -n \
  --arg project_id "$project_id" --arg summary "E2E article $run_suffix" \
  '{project: {id: $project_id}, summary: $summary, content: "E2E article body"}')")
article_id=$(printf '%s' "$article_response" | jq -er '.id')

agile_response=$(api POST '/api/agiles?fields=id,name' "$(jq -n \
  --arg name "E2E agile $run_suffix" --arg project_id "$project_id" \
  '{name: $name, projects: [{id: $project_id}]}')")
agile_id=$(printf '%s' "$agile_response" | jq -er '.id')

api POST "/api/issues/$first_issue_id/timeTracking/workItems?fields=id,text,duration(minutes),type(id,name)" \
  "$(jq -n --arg text "E2E work item $run_suffix" --arg type_id "$work_item_type_id" \
    '{text: $text, date: (now * 1000 | floor), duration: {minutes: 60}, type: {id: $type_id}}')" >/dev/null

# Discover the plugin schema before table-specific checks.
assert_count "schema discovery" \
  "select count(*)::int as result from information_schema.tables where table_schema = 'youtrack' and table_name like 'youtrack_%';" 1
assert_count "project identifier filter" \
  "select count(*)::int as result from youtrack.youtrack_project where id = '$project_id';"
assert_count "issue identifier filter/pushdown" \
  "select count(*)::int as result from youtrack.youtrack_issue where id_readable = '$first_issue_readable';"
assert_count "issue query pushdown" \
  "select count(*)::int as result from youtrack.youtrack_issue where query = 'project: $project_short_name';" 101
assert_count "pagination" \
  "select count(*)::int as result from youtrack.youtrack_issue where project_id = '$project_id';" 101
assert_count "null preservation" \
  "select count(*)::int as result from youtrack.youtrack_issue where id = '$first_issue_id' and description is null;"
assert_count "project/issue join" \
  "select count(*)::int as result from youtrack.youtrack_project p join youtrack.youtrack_issue i on i.project_id = p.id where p.id = '$project_id';" 101
assert_count "comment" \
  "select count(*)::int as result from youtrack.youtrack_issue_comment where issue_id = '$first_issue_id' and id = '$comment_id';"
assert_count "tag" "select count(*)::int as result from youtrack.youtrack_tag where id = '$tag_id';"
assert_count "saved query" "select count(*)::int as result from youtrack.youtrack_saved_query where id = '$saved_query_id';"
assert_count "article" "select count(*)::int as result from youtrack.youtrack_article where id = '$article_id';"
assert_count "agile" "select count(*)::int as result from youtrack.youtrack_agile where id = '$agile_id';"
assert_count "work item" \
  "select count(*)::int as result from youtrack.youtrack_issue_work_item where issue_id = '$first_issue_id';"

# The same binary is loaded as two named connections; both schemas must remain usable.
assert_count "primary connection" "select count(*)::int as result from youtrack.youtrack_project where id = '$project_id';"
assert_count "secondary connection isolation" "select count(*)::int as result from youtrack_secondary.youtrack_project where id = '$project_id';"

api GET '/api/users?fields=id&$top=1' >/dev/null || fail "current /api/users endpoint unavailable"
api GET '/api/groups?fields=id&$top=1' >/dev/null || fail "current /api/groups endpoint unavailable"
assert_count "user" "select count(*)::int as result from youtrack.youtrack_user;"
assert_count "group" "select count(*)::int as result from youtrack.youtrack_group;"

invalid_log="$state_dir/invalid-token.log"
if sql_json "select count(*) from youtrack_invalid.youtrack_project;" >"$invalid_log" 2>&1; then
  fail "invalid-token query unexpectedly succeeded"
fi
unavailable_log="$state_dir/unavailable-server.log"
if sql_json "select count(*) from youtrack_unavailable.youtrack_project;" >"$unavailable_log" 2>&1; then
  fail "unavailable-server query unexpectedly succeeded"
fi
for diagnostic_log in "$invalid_log" "$unavailable_log"; do
  if grep -F "$YOUTRACK_TOKEN" "$diagnostic_log" >/dev/null; then
    rm -f "$diagnostic_log"
    fail "valid token leaked in negative-test diagnostics"
  fi
done

api DELETE "/api/agiles/$agile_id" >/dev/null; agile_id=
api DELETE "/api/savedQueries/$saved_query_id" >/dev/null; saved_query_id=
api DELETE "/api/tags/$tag_id" >/dev/null; tag_id=
api DELETE "/api/admin/projects/$project_id" >/dev/null
status=$(curl --silent --output /dev/null --write-out '%{http_code}' \
  -H "Authorization: Bearer $YOUTRACK_TOKEN" "$base_url/api/admin/projects/$project_id")
[ "$status" = 404 ] || fail "seeded project still exists after cleanup (HTTP $status)"
project_id=

printf '%s\n' "e2e: real YouTrack API and Steampipe SQL checks passed"
