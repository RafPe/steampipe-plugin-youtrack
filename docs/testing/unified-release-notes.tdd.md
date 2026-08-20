# Curated release notes — TDD evidence

## Journey

A release reader sees the reviewed Changie section, not an uncurated commit
list, while the existing review, E2E, immutable-tag, and artifact gates remain
unchanged.

## Evidence

| Guarantee | Type | RED | GREEN |
| --- | --- | --- | --- |
| GoReleaser consumes the batched Changie section | Contract | `release-contract-check.sh` failed: `C4: GoReleaser must receive the curated changelog section` | Contract passed with C4 enabled |
| Curated notes are staged outside the work tree | Contract | Restoring the in-tree `notes="release-notes.md"` failed: `C4: the curated notes must be staged outside the work tree ($RUNNER_TEMP)` | Contract passed with the notes staged in `$RUNNER_TEMP` |
| README names the explicit preparation gate | Contract/docs | Failed with the same new C4 contract | README now names **Prepare Release**; contract passed |
| Release workflow remains syntactically valid | Integration/static | N/A | `actionlint -ignore SC2129 .github/workflows/*.yml`: PASS; SC2129 is a pre-existing style warning |
| Release helper behavior remains valid | Unit | N/A | `go test ./internal/release ./cmd/releasectl`: PASS |

The existing release workflow continues to run its self-contained YouTrack E2E
job before tagging. This change only supplies `.changes/${VERSION}.md` to
GoReleaser through `--release-notes`.
