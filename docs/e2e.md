# End-to-end tests

The E2E harness uses the pinned YouTrack Server `2026.1.13874` container and
the official Steampipe `2.3.2` release binary. Turbot stopped publishing its
official Steampipe container after `0.22.0`, so the harness downloads the
platform-specific `2.3.2` archive and verifies its SHA-256 checksum before use.
The cached binary runs with an isolated Steampipe install directory. The test
builds the plugin from the current checkout, installs that exact binary, seeds
resources through supported YouTrack REST APIs, and verifies them through real
Steampipe SQL.

## One-time YouTrack setup

A new YouTrack Server data volume always starts the JetBrains configuration
wizard. JetBrains does not expose a supported, deterministic API for completing
the wizard or minting its first permanent token. This one-time boundary is
therefore intentionally manual. This follows JetBrains' official
[Docker installation instructions](https://www.jetbrains.com/help/youtrack/server/youtrack-docker-installation.html)
and [permanent-token procedure](https://www.jetbrains.com/help/youtrack/server/manage-permanent-token.html):

1. Start YouTrack with `docker compose up -d youtrack`.
2. Read the wizard URL with `docker compose logs youtrack`. It contains the
   one-time `wizard_token`.
3. Open that URL, choose **Set up**, use `http://localhost:18080` as the base
   URL, and create the administrator account. Do not reuse a production
   password.
4. In the administrator profile, create a permanent token with YouTrack and
   YouTrack Administration scopes. Copy it when displayed; YouTrack cannot
   display it again.

The named YouTrack volumes preserve this setup between test runs. The token is
never committed or written into generated configuration: the mode `0600`
Steampipe connection file refers to `YOUTRACK_TOKEN` through `env()` and is
removed when the test exits.

## Run

Export the token only for the current shell, then run the harness:

```sh
read -r -s YOUTRACK_TOKEN
export YOUTRACK_TOKEN
tests/e2e/run.sh
unset YOUTRACK_TOKEN
```

Prerequisites are Docker Compose, Go, curl, and jq. The run fails before making
changes if any prerequisite or the token is missing. Every run uses a unique
project name and creates 101 issues to cross the default API page boundary. It
also seeds supported REST resources for comments, tags, saved queries,
articles, agile boards, and work items. The harness creates a temporary global
work-item type, enables project time tracking, creates a typed work item, and
removes the type during cleanup. Real Steampipe SQL checks cover schema discovery, identifiers, issue
query pushdown, pagination, null values, joins, two named connections, invalid
credentials, and an unavailable server. The harness deletes all resources it
created, stops the Compose services, checks that no project containers remain,
and removes local credentials and plugin artifacts. Negative-test logs remain
under the ignored `.state` directory for diagnostics. The initialized YouTrack
volumes remain so the manual setup is not repeated.

### Deterministic setup and preflights

YouTrack Server 2026.1 provides supported current collection endpoints at
`GET /api/users` and `GET /api/groups`. The harness requires both endpoints and
never calls deprecated `/hub` internals; `GET /api/users/me` verifies the token
and selects the new project's leader.

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
