# Security policy

## Supported versions

Until the first tagged release, security fixes are applied to the `main`
branch. After the first release, only the latest minor release will receive
security fixes. Users should upgrade to the newest available release before
reporting a vulnerability.

## Reporting a vulnerability

Report vulnerabilities through
[GitHub private vulnerability reporting](https://github.com/RafPe/steampipe-youtrack/security/advisories/new).
Do not include tokens, authorization headers, private YouTrack data, or other
credentials in a report. Use synthetic values and redact diagnostic output.

Please include:

- The affected plugin and Steampipe versions.
- The affected operating system and architecture.
- A minimal reproduction using synthetic data.
- The security impact and any known mitigations.

Do not disclose a vulnerability in a public issue, discussion, or pull
request. The maintainer will acknowledge the report, investigate it, and
coordinate remediation and disclosure through the private advisory.

## Security model

The plugin is read-only but acts with the permissions of its configured
YouTrack permanent token. Use a dedicated, least-privilege token, keep it in an
environment variable or secret manager, and rotate it immediately if it may
have been exposed. The plugin must never log or return the token.

Reports about upstream vulnerabilities in Steampipe, Go, or YouTrack should be
sent to the corresponding upstream project unless this plugin makes the issue
independently exploitable.

## Supply chain

Every third-party GitHub Action used in CI and release workflows is pinned
to a full 40-character commit SHA (with the release tag kept only as a
comment), not a mutable tag or branch. Release tool versions (Changie,
GoReleaser, syft) are pinned the same way, invoked through `go run` so
nothing is preinstalled or trusted implicitly.

Each release publishes, alongside the plugin archives: a SHA-256
`checksums.txt`, a per-archive SPDX SBOM, and a GitHub build-provenance
attestation covering the archives. See [Verifying release
artifacts](docs/releases.md#verifying-release-artifacts) for the exact
`sha256sum`, `gh attestation verify`, and SBOM-inspection commands.
