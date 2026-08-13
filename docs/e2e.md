# End-to-end tests

The E2E harness uses the pinned YouTrack Server `2026.1.13874` container and
the official Steampipe `2.3.2` release binary. Turbot stopped publishing its
official Steampipe container after `0.22.0`, so the harness downloads the
platform-specific `2.3.2` archive and verifies its SHA-256 checksum before use.
The cached binary runs with an isolated Steampipe install directory. The test
builds the plugin from the current checkout, installs that exact binary, seeds
resources through supported YouTrack REST APIs, and verifies them through real
Steampipe SQL.

## No stored credential: automated provisioning

No real YouTrack token is ever stored as a secret. By default, `run.sh`
provisions its own throwaway YouTrack instance: `tests/e2e/provision.sh`
drives the pinned image's first-run configuration wizard through its REST
API (empirically discovered against this exact digest-pinned image; see
`provision.sh`'s header comment for the endpoint sequence), creates a
randomly generated administrator account that is never printed or written
to disk, and mints a permanent token via the Hub REST API. The instance and
its volumes are destroyed at the end of the run.

JetBrains does not expose a *supported* wizard API, but because
`docker-compose.yml` pins the image to an exact digest
(`jetbrains/youtrack:2026.1.13874@sha256:...`), automating this specific
version's wizard endpoints is deterministic in practice. If a future digest
bump changes the wizard flow, `provision.sh` fails loudly with a diagnostic
rather than hanging or silently degrading -- see its header comment, which
documents the coupling between the pin and the script.

`provision.sh`'s output contract: on success, its stdout is *only* the
minted token (one line); everything else -- progress, diagnostics, errors --
goes to stderr. Callers capture it with command substitution:

```sh
YOUTRACK_TOKEN=$(tests/e2e/provision.sh)
```

For local iteration, `E2E_REUSE=1 tests/e2e/provision.sh` skips the
first-run wizard on a second call against the same volumes (it persists the
generated admin credentials under the gitignored `tests/e2e/.state/` and
mints a fresh token from them instead), which is much faster than a full
fresh-recreate. CI never sets this: every CI run gets a genuinely fresh
instance.

The token is never committed or written into generated configuration: the
mode `0600` Steampipe connection file omits the `token` argument so the
plugin reads it from the `YOUTRACK_TOKEN` environment variable, and the file
is removed when the test exits.

### Manual override

Set `YOUTRACK_TOKEN` yourself to skip automated provisioning entirely --
useful against a long-lived local dev instance you've already configured by
hand. This follows JetBrains' official
[Docker installation instructions](https://www.jetbrains.com/help/youtrack/server/youtrack-docker-installation.html)
and [permanent-token procedure](https://www.jetbrains.com/help/youtrack/server/manage-permanent-token.html):
start YouTrack, read the wizard URL (with its one-time `wizard_token`) from
`docker compose logs youtrack`, complete the wizard in a browser, and mint a
permanent token with YouTrack and YouTrack Administration scopes from the
administrator profile. When `YOUTRACK_TOKEN` is already set, `run.sh` uses
it as-is and -- unlike the automated-provisioning path -- does not destroy
the compose volumes on exit, so a manually configured instance survives
between runs.

## Run

With automated provisioning (default), just run the harness:

```sh
tests/e2e/run.sh
```

To use a token you minted yourself instead, export it for the current shell
first:

```sh
read -r -s YOUTRACK_TOKEN
export YOUTRACK_TOKEN
tests/e2e/run.sh
unset YOUTRACK_TOKEN
```

Prerequisites are Docker Compose, Go, curl, and jq. The run fails before making
changes if any prerequisite is missing, or if automated provisioning fails and
no `YOUTRACK_TOKEN` override was given. Every run uses a unique project name
and creates 101 issues to cross the default API page boundary. It
also seeds supported REST resources for comments, tags, saved queries,
articles, agile boards, and work items. The harness creates a temporary global
work-item type, enables project time tracking, creates a typed work item, and
removes the type during cleanup. Real Steampipe SQL checks cover schema discovery, identifiers, issue
query pushdown, pagination, null values, joins, two named connections, invalid
credentials, and an unavailable server. The harness deletes all resources it
created, stops the Compose services, checks that no project containers remain,
and removes local credentials and plugin artifacts. Negative-test logs remain
under the ignored `.state` directory for diagnostics. With automated
provisioning, the throwaway YouTrack volumes are destroyed along with the
containers, so the next run provisions a genuinely fresh instance (use
`E2E_REUSE=1` to keep them for faster local iteration instead). With a manual
`YOUTRACK_TOKEN` override, the volumes are left alone so the manually
configured instance is not repeated.

### Deterministic setup and preflights

YouTrack Server 2026.1 provides supported current collection endpoints at
`GET /api/users` and `GET /api/groups`. `run.sh` requires both endpoints and
never calls `/hub` internals itself; `GET /api/users/me` verifies the token
and selects the new project's leader. (`provision.sh` is a separate concern:
it does call the Hub REST API, because minting a permanent token is
inherently a Hub operation -- see its header comment.)

Work-item creation requires project time tracking and an attached work-item
type. The harness configures both through the current `/api/admin` resources;
failure to configure or query work items is a test failure rather than a skip.
The E2E permanent token therefore needs project-update and work-item-type
administration permissions in addition to the scopes used for other resources.

The two negative connection checks require the plugin to return authentication
and network errors without leaking the valid token. They intentionally use a
fixed non-secret invalid token and loopback discard port `9`; no external
unavailable host is contacted.

To validate the harness without starting the large images, run
`tests/e2e/check.sh`. To remove all generated state, including the initialized
YouTrack database and its tokens, run `tests/e2e/cleanup.sh`. The next E2E run
will require the one-time wizard and a newly generated permanent token.

## Running E2E on a pull request

E2E does not run on every pull request by default -- it's slow and starts a
real ~2GB container. Apply the `e2e` label to a pull request to opt it in:
every push to that pull request then runs the full suite (`.github/workflows/e2e.yml`'s
`pull_request` trigger), gated on the label being present at the time of each
push. Remove the label to stop; a pull request without the label is
unaffected. Since E2E provisions its own throwaway YouTrack instance and
uses no secret, this is safe to enable on a pull request from a fork -- there
is no stored credential for it to expose.
