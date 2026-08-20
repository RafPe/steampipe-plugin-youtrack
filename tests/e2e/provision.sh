#!/bin/sh
# Provisions a fresh, throwaway YouTrack instance and mints a permanent
# token for it, without ever storing a real YouTrack credential anywhere
# persistent. Drives the pinned image's first-run configuration wizard
# through its REST API (there is no supported wizard API, but the image is
# digest-pinned in docker-compose.yml, so this exact version's wizard/Hub
# endpoints are stable) and then mints a permanent token via the Hub REST
# API. See docs/e2e.md for the discovered flow and the token-handoff
# contract this script implements.
#
# Coupling: this script and the image digest pin in docker-compose.yml move
# together. If the wizard endpoints below start failing after a digest
# bump, the wizard flow has changed upstream and this script needs
# re-discovery against the new image before the pin can be updated.
#
# Contract: on success, the ONLY thing written to stdout is the minted
# permanent token (one line, no trailing content). All progress and
# diagnostics go to stderr. Callers capture it with command substitution:
#   YOUTRACK_TOKEN=$(tests/e2e/provision.sh)
# On failure, nothing is printed to stdout and the exit status is non-zero.
#
# Modes:
#   - default: destroys and recreates the youtrack service's volumes before
#     provisioning, so every run starts from a known-empty instance
#     (deterministic; this is what CI and `make e2e` use).
#   - E2E_REUSE=1: keeps existing volumes. If the instance is already
#     configured (from a previous provision.sh run in this mode), reuses
#     the admin credentials persisted under tests/e2e/.state to mint a new
#     token without repeating the wizard. If the instance is unconfigured
#     (fresh volumes), runs the wizard as normal and persists the generated
#     admin credentials for next time. Intended for local iteration only;
#     never used in CI.
#
# Tuning: E2E_STARTUP_TIMEOUT (seconds, default 600) caps how long to wait
# for YouTrack to come back up after the wizard finishes. Raise it if a
# slow or loaded machine trips the ceiling; it only bounds a poll loop that
# exits as soon as the instance is ready.
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
state_dir="$repo_dir/tests/e2e/.state"
credentials_file="$state_dir/provision-admin.json"
base_url=${YOUTRACK_URL:-http://localhost:${YOUTRACK_PORT:-18080}}
reuse=${E2E_REUSE:-0}

compose() {
  docker compose --project-directory "$repo_dir" -f "$repo_dir/docker-compose.yml" "$@"
}

fail() {
  printf '%s\n' "provision: $*" >&2
  exit 1
}

for command_name in docker curl jq; do
  command -v "$command_name" >/dev/null 2>&1 || fail "required command not found: $command_name"
done

random_string() {
  # $1 = character count. Alphanumeric only, so callers never need to
  # JSON-escape the result.
  LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c "$1"
}

# All wizard calls carry the one-time wizard token as a query parameter,
# matching the installer SPA's own $http interceptor (see task-6-report.md
# for how this was discovered from the wizard's main.js).
wizard_get() {
  curl --fail --silent --show-error --max-time 30 \
    "$base_url$1?wizardToken=$wizard_token"
}

wizard_post() {
  path=$1
  data=$2
  curl --fail --silent --show-error --max-time 30 -X POST \
    -H 'Content-Type: application/json' \
    --data "$data" \
    "$base_url$path?wizardToken=$wizard_token"
}

mkdir -p "$state_dir"
chmod 700 "$state_dir"

if [ "$reuse" != "1" ]; then
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -f "$credentials_file"
fi

compose up -d --wait youtrack >&2

# Distinguish "unconfigured" (wizard mode) from "already configured" (real
# backend) without a token in hand: an unauthenticated GET on the wizard
# steps endpoint returns 403 INVALID_AUTH_TOKEN before setup and 401
# Unauthorized (the real backend's generic auth error) after it.
probe_file=$(mktemp "$state_dir/wizard-probe.XXXXXX")
probe_status=$(curl --silent --show-error --max-time 30 --output "$probe_file" \
  --write-out '%{http_code}' "$base_url/api/wizard/steps" 2>/dev/null) || probe_status=000
probe_body=$(cat "$probe_file" 2>/dev/null || true)
rm -f "$probe_file"

case "$probe_status" in
  403)
    already_configured=0
    ;;
  401)
    already_configured=1
    ;;
  *)
    fail "unexpected response probing wizard state (HTTP $probe_status); the pinned image's wizard flow may have changed. Body: $probe_body"
    ;;
esac

if [ "$already_configured" -eq 1 ]; then
  [ "$reuse" = "1" ] || fail "instance is already configured but E2E_REUSE was not set; this should not happen after a fresh recreate"
  [ -f "$credentials_file" ] || fail "instance is already configured but no persisted admin credentials were found at $credentials_file; run tests/e2e/cleanup.sh and retry, or unset E2E_REUSE for a fresh instance"
  admin_login=$(jq -er '.login' "$credentials_file") || fail "could not read admin login from $credentials_file"
  admin_password=$(jq -er '.password' "$credentials_file") || fail "could not read admin password from $credentials_file"
else
  # --- Drive the first-run configuration wizard ---
  wizard_token=""
  attempt=0
  while [ "$attempt" -lt 30 ]; do
    wizard_token=$(compose logs youtrack 2>&1 | grep -oE 'wizard_token=[A-Za-z0-9_-]+' | tail -1 | cut -d= -f2 || true)
    [ -n "$wizard_token" ] && break
    attempt=$((attempt + 1))
    sleep 2
  done
  [ -n "$wizard_token" ] || fail "could not find the one-time wizard_token in 'docker compose logs youtrack' after 60s; the pinned image's startup log format may have changed"

  admin_login="e2e-$(random_string 8)"
  admin_password=$(random_string 32)
  [ -n "$admin_password" ] || fail "failed to generate a random admin password"

  hub_settings_body=$(jq -n \
    --arg rootUser "$admin_login" \
    --arg rootPassword "$admin_password" \
    '{rootUser: $rootUser, rootPassword: $rootPassword, disableInternalHub: false, allowAnonymousAccess: false}')
  # Do not log the response: it echoes rootPassword back in plaintext.
  wizard_post /api/wizard/hub-settings "$hub_settings_body" >/dev/null ||
    fail "wizard hub-settings step failed; the pinned image's wizard API may have changed (see docs/e2e.md)"

  steps_after=$(wizard_get /api/wizard/steps) || fail "could not read wizard steps after hub-settings"
  hub_completed=$(printf '%s' "$steps_after" | jq -er '.[] | select(.id == "hub-settings") | .completed') ||
    fail "wizard steps response did not include a hub-settings entry; the pinned image's wizard flow may have changed"
  [ "$hub_completed" = "true" ] || fail "wizard did not mark hub-settings as completed after submitting admin credentials"

  # This is the installer SPA's own "Finish" action (see task-6-report.md):
  # POST /wizard/wait/dump triggers the backend to leave wizard mode and
  # start the real application.
  wizard_post /api/wizard/wait/dump '{}' >/dev/null ||
    fail "wizard finish step (wizard/wait/dump) failed; the pinned image's wizard API may have changed"

  # After finishing, the backend restarts internally and serves a 503
  # "warming up" page until it is ready; poll /api/users/me until it flips
  # to the real backend's 401 Unauthorized.
  #
  # The ceiling is wall-clock rather than an iteration count because each
  # probe can itself burn up to its --max-time against an unresponsive
  # backend, which shrinks a counted loop's real budget exactly when the
  # instance is slowest. It is also deliberately generous (default 10min,
  # well inside e2e.yml's 30min job timeout): this restart is JVM- and
  # disk-bound, so its duration tracks runner load rather than anything in
  # this repo. A 180s ceiling here failed the v0.2.0 release on a cold CI
  # runner even though the same commit provisioned in 55s three days
  # earlier.
  startup_timeout=${E2E_STARTUP_TIMEOUT:-600}
  ready=0
  status=000
  deadline=$(($(date +%s) + startup_timeout))
  while [ "$(date +%s)" -lt "$deadline" ]; do
    status=$(curl --silent --show-error --max-time 10 --output /dev/null \
      --write-out '%{http_code}' "$base_url/api/users/me" 2>/dev/null) || status=000
    if [ "$status" = "401" ]; then
      ready=1
      break
    fi
    sleep 2
  done
  if [ "$ready" -ne 1 ]; then
    # Say what the backend was actually reporting, so the next failure is
    # diagnosable from the log alone: a final 503 means it was still warming
    # up and the ceiling is genuinely too low, whereas 000 (connection
    # refused) means the container went away and no ceiling would have
    # helped. The CI job tears the stack down on failure, so dump the
    # container's own log here while it still exists.
    printf '%s\n' "provision: last status from /api/users/me was $status; recent youtrack container logs follow" >&2
    compose logs --tail 50 youtrack >&2 || true
    fail "YouTrack did not finish starting after the wizard completed (waited ${startup_timeout}s, last HTTP status $status); the pinned image's startup behavior may have changed"
  fi

  if [ "$reuse" = "1" ]; then
    umask 077
    jq -n --arg login "$admin_login" --arg password "$admin_password" \
      '{login: $login, password: $password}' >"$credentials_file"
    chmod 600 "$credentials_file"
  fi
fi

# --- Mint a permanent token via the Hub REST API ---
ring_id=$(curl --fail --silent --show-error --max-time 30 -u "$admin_login:$admin_password" \
  "$base_url/hub/api/rest/users/me?fields=id" | jq -er '.id') ||
  fail "could not resolve the admin user's Hub ring ID; admin authentication may have failed"

services_json=$(curl --fail --silent --show-error --max-time 30 -u "$admin_login:$admin_password" \
  "$base_url/hub/api/rest/services?fields=id,name,applicationName") ||
  fail "could not list Hub services to resolve token scopes"

admin_service_id=$(printf '%s' "$services_json" | jq -er '[.services[] | select(.name == "YouTrack Administration")] | if length == 1 then .[0].id else error("expected exactly one YouTrack Administration service") end') ||
  fail "could not resolve the YouTrack Administration service scope; the pinned image's Hub service topology may have changed"
youtrack_service_id=$(printf '%s' "$services_json" | jq -er '[.services[] | select(.applicationName == "YouTrack")] | if length == 1 then .[0].id else error("expected exactly one YouTrack service") end') ||
  fail "could not resolve the YouTrack service scope; the pinned image's Hub service topology may have changed"

token_body=$(jq -n --arg admin_id "$admin_service_id" --arg yt_id "$youtrack_service_id" \
  '{name: "steampipe-e2e-provision", scope: [{id: $admin_id}, {id: $yt_id}]}')
# Do not log the response: it contains the minted token in plaintext.
token=$(curl --fail --silent --show-error --max-time 30 -u "$admin_login:$admin_password" \
  -H 'Content-Type: application/json' -X POST --data "$token_body" \
  "$base_url/hub/api/rest/users/$ring_id/permanenttokens?fields=token" | jq -er '.token') ||
  fail "could not mint a permanent token via the Hub REST API"

[ -n "$token" ] || fail "minted token was empty"

# Sanity-check the token actually authenticates against the real API before
# handing it back, so a caller never receives a token that silently doesn't
# work.
verify_status=$(curl --silent --show-error --max-time 30 --output /dev/null \
  --write-out '%{http_code}' -H "Authorization: Bearer $token" "$base_url/api/users/me") || verify_status=000
[ "$verify_status" = "200" ] || fail "minted token failed to authenticate against /api/users/me (HTTP $verify_status)"

printf '%s: provisioning complete\n' "provision" >&2
printf '%s\n' "$token"
