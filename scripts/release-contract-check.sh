#!/bin/sh
set -eu

# Offline guard against the workflow<->tool contract seams that broke in
# the final whole-branch review and the first live run (see
# task-4-report.md, "Final-review fix wave", "Post-advisor corrections",
# and "Additional fix" sections): a fragment file path expected on disk
# that isn't there, an attacker-controlled fragment path that could
# traverse outside its scratch directory, a same-repo PR unable to be
# validated by a releasectl built from a stale base commit, a `gh api`
# call whose HTTP method silently changed, and a filename one tool writes
# that another tool asserts under a different name. Each check below
# exercises the exact runtime shape of its fix, not just "the code
# compiles" -- run locally and in CI (ci.yml's release-config job), no
# network access required, no repository files mutated.

# Pinned changie version, matching .changie.yaml / scripts/changelog-check.sh.
changie_version="v1.25.2"

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

fail() {
	printf '%s\n' "release-contract-check: $*" >&2
	exit 1
}

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/steampipe-youtrack-release-contract-check.XXXXXX")"
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM

cd "$repo_root"

# --- 1. C1: pr-release-metadata.yml's fetched-fragment layout --------------
#
# Reproduces exactly what the fixed workflow does: build releasectl once
# (standing in for the trusted base.sha build), then mirror the fragments
# directory's normal repo-relative layout (.changes/unreleased/<name>.yaml)
# under a scratch root and invoke the prebuilt binary from inside it with
# --fragments-dir .changes/unreleased. This is the layout `fragmentCandidates`
# (internal/release/fragments.go) requires: changed_files entries are always
# repo-relative (absolute paths are rejected outright), so --fragments-dir
# must stay relative and match that prefix textually -- an absolute
# --fragments-dir, or a fetched file placed anywhere else, yields zero
# candidates and "requires at least one changelog fragment" instead of
# actually reading what was fetched. That failure mode is exactly what the
# "invalid fetched fragment" case below is designed to catch: if it fired
# for the wrong reason (no candidates found, rather than the fragment's own
# unknown-kind error), the layout would be silently validating nothing.
releasectl_bin="$work_dir/releasectl"
go build -o "$releasectl_bin" ./cmd/releasectl

fragments_scratch="$work_dir/pr-fragments"
mkdir -p "$fragments_scratch/.changes/unreleased"

printf 'kind: Added\nbody: A fixture fragment fetched from PR HEAD.\n' \
	>"$fragments_scratch/.changes/unreleased/Added-1.yaml"
printf 'kind: Bogus\nbody: An invalid-kind fixture fragment fetched from PR HEAD.\n' \
	>"$fragments_scratch/.changes/unreleased/Added-2.yaml"

valid_payload='{"labels":["release/patch"],"head_branch":"feat/x","head_repo":"someone/steampipe-youtrack","changed_files":[".changes/unreleased/Added-1.yaml"]}'
valid_output="$(
	cd "$fragments_scratch" &&
		set +e
	printf '%s' "$valid_payload" | "$releasectl_bin" validate-pr \
		--input - --trusted-repo RafPe/steampipe-plugin-youtrack --fragments-dir .changes/unreleased 2>&1
	echo "EXIT:$?"
)"
valid_status="$(printf '%s\n' "$valid_output" | sed -n 's/^EXIT://p')"
[ "$valid_status" = "0" ] ||
	fail "C1: a valid fetched fragment should pass, got exit $valid_status: $valid_output"
case "$valid_output" in
*"ok: release/patch with 1 changelog fragment(s)"*) : ;;
*) fail "C1: valid fetched fragment did not produce the expected success message: $valid_output" ;;
esac

invalid_payload='{"labels":["release/patch"],"head_branch":"feat/x","head_repo":"someone/steampipe-youtrack","changed_files":[".changes/unreleased/Added-2.yaml"]}'
invalid_output="$(
	cd "$fragments_scratch" &&
		set +e
	printf '%s' "$invalid_payload" | "$releasectl_bin" validate-pr \
		--input - --trusted-repo RafPe/steampipe-plugin-youtrack --fragments-dir .changes/unreleased 2>&1
	echo "EXIT:$?"
)"
invalid_status="$(printf '%s\n' "$invalid_output" | sed -n 's/^EXIT://p')"
[ "$invalid_status" = "1" ] ||
	fail "C1: an invalid fetched fragment should fail (exit 1), got exit $invalid_status: $invalid_output"
# The discriminating assertion: it must fail on the fragment's OWN content
# (unknown kind), not on "requires at least one changelog fragment" -- the
# latter would mean the layout silently found nothing to validate.
case "$invalid_output" in
*"unknown kind"*) : ;;
*"requires at least one changelog fragment"*)
	fail "C1: invalid fragment was not found at all (layout mismatch: fragments-dir/changed_files prefix did not resolve to the fetched file) -- got: $invalid_output"
	;;
*) fail "C1: invalid fetched fragment failed for an unexpected reason: $invalid_output" ;;
esac

# --- 1b. C1: fetch-loop path traversal guard --------------------------------
#
# `filename` in pr-release-metadata.yml's fetch loop comes straight from the
# PR files API and is fork-controllable. The jq prefilter there
# (startswith(".changes/unreleased/") and endswith(".yaml")) does NOT stop a
# traversal shape like ".changes/unreleased/../../escape.yaml" -- it
# textually starts with the prefix and ends in ".yaml", so it passes. This
# mirrors that workflow's two-layer guard (see its twin, same comments,
# around `while IFS= read -r entry` in pr-release-metadata.yml) against a
# fixture entry with exactly that shape, and asserts (a) it hard-fails and
# (b) nothing was written outside the scratch directory. Keep this in sync
# with the workflow if either changes.
traversal_scratch="$work_dir/traversal-guard"
mkdir -p "$traversal_scratch/.changes/unreleased"

traversal_status=0
(
	set -eu
	filename=".changes/unreleased/../../escape.yaml"

	# Exact copy of the workflow's guard: see the comment there for why
	# $newline must be built this way rather than "$(printf '\n')".
	newline="$(printf 'X\nX')"
	newline="${newline#X}"
	newline="${newline%X}"
	case "$filename" in
	*..*|/*|*"$newline"*)
		echo "::error::suspicious fragment path from PR files API: $filename" >&2
		exit 1
		;;
	esac

	target="$traversal_scratch/$filename"
	target_dir="$(dirname "$target")"
	mkdir -p "$target_dir"

	resolved_target_dir="$(cd "$target_dir" && pwd -P)"
	resolved_scratch="$(cd "$traversal_scratch" && pwd -P)"
	case "$resolved_target_dir" in
	"$resolved_scratch" | "$resolved_scratch"/*) : ;;
	*)
		echo "::error::fragment path escapes the scratch directory: $filename" >&2
		exit 1
		;;
	esac

	printf 'kind: Added\nbody: should never be written.\n' >"$target"
) || traversal_status=$?

[ "$traversal_status" != "0" ] ||
	fail "C1: traversal filename '.changes/unreleased/../../escape.yaml' should have been rejected but the guard let it through"

if find "$work_dir" -name 'escape.yaml' | grep -q .; then
	find "$work_dir" -name 'escape.yaml' >&2
	fail "C1: traversal filename escaped the scratch directory -- a file landed outside $traversal_scratch"
fi

# Sanity check the guard isn't over-broad: a legitimate filename (no ".."
# or leading "/", no embedded newline) must still be accepted.
(
	set -eu
	filename=".changes/unreleased/Added-legit.yaml"
	newline="$(printf 'X\nX')"
	newline="${newline#X}"
	newline="${newline%X}"
	case "$filename" in
	*..*|/*|*"$newline"*)
		echo "::error::suspicious fragment path from PR files API: $filename" >&2
		exit 1
		;;
	esac
	target="$traversal_scratch/$filename"
	target_dir="$(dirname "$target")"
	mkdir -p "$target_dir"
	resolved_target_dir="$(cd "$target_dir" && pwd -P)"
	resolved_scratch="$(cd "$traversal_scratch" && pwd -P)"
	case "$resolved_target_dir" in
	"$resolved_scratch" | "$resolved_scratch"/*) : ;;
	*)
		echo "::error::fragment path escapes the scratch directory: $filename" >&2
		exit 1
		;;
	esac
	printf 'kind: Added\nbody: legitimate fixture.\n' >"$target"
) || fail "C1: a legitimate, non-traversal filename was incorrectly rejected by the guard"
[ -f "$traversal_scratch/.changes/unreleased/Added-legit.yaml" ] ||
	fail "C1: legitimate filename was accepted by the guard but never written"

# --- 1c. C1 bootstrap: same-repo path validates against HEAD's CLI ---------
#
# Live failure (https://github.com/RafPe/steampipe-plugin-youtrack/pull/5): a
# same-repo PR that adds a new releasectl flag (--trusted-repo, in that
# case) cannot be validated by a releasectl built from base.sha, because
# base necessarily predates the PR's own CLI change -- the base-built
# binary rejected the flag with "flag provided but not defined". The fix
# is pr-release-metadata.yml's trust split: same-repo PRs build+validate
# from the PR's own merge ref, not base.sha.
#
# This proves the *mechanism* generalizes, not just today's existing
# --trusted-repo flag (which already exists in both trees here and so
# wouldn't prove anything about staleness): build one releasectl from the
# real, unmodified source (standing in for "base", which predates any
# same-repo PR's own change) and a second from a minimal copy patched with
# a brand-new synthetic flag (standing in for "head"/the PR's merge ref).
# The base build must reject the new flag (reproducing the live failure
# shape); the head build must accept it (proving the same-repo path's
# build-from-head fix resolves this class of bug for ANY future CLI
# change, not just the one already fixed).
head_copy="$work_dir/head-copy"
mkdir -p "$head_copy"
cp "$repo_root/go.mod" "$repo_root/go.sum" "$head_copy/"
cp -r "$repo_root/cmd" "$repo_root/internal" "$head_copy/"

sed 's#fragmentsDir := fs.String("fragments-dir", defaultFragmentsDir, "changelog fragments directory")#&\n\tsimulatedNewFlag := fs.String("simulated-new-flag", "", "contract-check fixture only")\n\t_ = simulatedNewFlag#' \
	"$head_copy/internal/release/run.go" >"$head_copy/internal/release/run.go.patched"
mv "$head_copy/internal/release/run.go.patched" "$head_copy/internal/release/run.go"
grep -q 'simulated-new-flag' "$head_copy/internal/release/run.go" ||
	fail "C1 bootstrap: failed to patch the head-copy fixture with a synthetic new flag -- internal/release/run.go's flag-registration line may have changed shape"

head_releasectl="$work_dir/releasectl-head"
if ! (cd "$head_copy" && go build -o "$head_releasectl" ./cmd/releasectl) >"$work_dir/head-build.log" 2>&1; then
	cat "$work_dir/head-build.log" >&2
	fail "C1 bootstrap: failed to build the head-copy fixture"
fi

base_output="$("$releasectl_bin" validate-pr --input /dev/null --trusted-repo x --simulated-new-flag y 2>&1)" || :
case "$base_output" in
*'flag provided but not defined'*'simulated-new-flag'*) : ;;
*) fail "C1 bootstrap: base build should have rejected the unknown flag (reproducing the PR #5 failure shape), got: $base_output" ;;
esac

head_output="$("$head_releasectl" validate-pr --input /dev/null --trusted-repo x --simulated-new-flag y 2>&1)" || :
case "$head_output" in
*'flag provided but not defined'*) fail "C1 bootstrap: same-repo (head) build should recognize --simulated-new-flag, but rejected it: $head_output" ;;
*) : ;;
esac

# --- 2. C2: prepare-release.yml's search/issues call must use -X GET -------
#
# `gh api` defaults to POST whenever -f/-F is present; POST /search/issues
# doesn't exist (404). Static assert only -- no network call, matching this
# script's offline contract.
prepare_release_workflow=".github/workflows/prepare-release.yml"
search_line="$(grep -n 'search/issues' "$prepare_release_workflow" | grep 'gh api' || true)"
[ -n "$search_line" ] ||
	fail "C2: could not find the gh api search/issues call in $prepare_release_workflow"
case "$search_line" in
*"-X GET"*) : ;;
*) fail "C2: gh api search/issues call is missing -X GET: $search_line" ;;
esac

# Regression guard for the sibling contract seam repaired earlier (Task 4,
# task-4-report.md): the exact-field jq projection that keeps the PRs
# payload sent to `releasectl next-version` matching its strict-decoded
# wire contract (extra fields like "title"/"url" fail with exit 2).
projection_line="$(grep -n 'prs_for_releasectl=' "$prepare_release_workflow" || true)"
[ -n "$projection_line" ] ||
	fail "C2: could not find the prs_for_releasectl jq projection in $prepare_release_workflow"
case "$projection_line" in
*'{number, labels, head_branch, head_repo}'*) : ;;
*) fail "C2: prs_for_releasectl projection no longer matches releasectl's wire contract: $projection_line" ;;
esac

# --- 3. C3: changie batch's actual output filename -------------------------
#
# changie names its batched fragment file after the v-prefixed version
# passed to `batch` (e.g. ".changes/v0.1.0.md"), not the un-prefixed
# heading it renders inside CHANGELOG.md. Proven directly against the real,
# pinned changie binary in a scratch fixture -- never against this
# checkout's own .changes/CHANGELOG.md.
changie_fixture_dir="$work_dir/changie-fixture"
mkdir -p "$changie_fixture_dir/changes/unreleased"
cp "$repo_root/.changes/header.tpl.md" "$changie_fixture_dir/changes/header.tpl.md"
printf 'kind: Added\nbody: Fixture fragment for release-contract-check.\n' \
	>"$changie_fixture_dir/changes/unreleased/Added-1.yaml"

# changesDir and changelogPath both redirected into the fixture: changesDir
# because that's where batch writes the versioned fragment file this check
# asserts on, changelogPath defensively so no changie invocation here can
# ever touch this checkout's real CHANGELOG.md, even though plain `batch`
# (unlike `merge`) does not write it.
fixture_config="$work_dir/fixture.changie.yaml"
sed -e "s#^changesDir:.*#changesDir: $changie_fixture_dir/changes#" \
	-e "s#^changelogPath:.*#changelogPath: $changie_fixture_dir/CHANGELOG.md#" \
	"$repo_root/.changie.yaml" >"$fixture_config"

test_version="v0.1.0"
if ! CHANGIE_CONFIG_PATH="$fixture_config" \
	go run "github.com/miniscruff/changie@${changie_version}" batch "$test_version" \
	>"$work_dir/changie-batch.log" 2>&1; then
	cat "$work_dir/changie-batch.log" >&2
	fail "C3: changie batch failed in the scratch fixture"
fi

expected_file="$changie_fixture_dir/changes/${test_version}.md"
[ -f "$expected_file" ] ||
	fail "C3: changie batch did not write the expected $expected_file -- release.yml's assert shape has drifted from changie's actual output"

release_workflow=".github/workflows/release.yml"
grep -q '\.changes/\${VERSION}\.md' "$release_workflow" ||
	fail "C3: $release_workflow no longer asserts the v-prefixed .changes/\${VERSION}.md filename"

# --- 4. C4: published notes come from the curated changelog ----------------
grep -q -- '--release-notes "\$notes"' "$release_workflow" ||
	fail "C4: GoReleaser must receive the curated changelog section as release notes"
# GoReleaser validates git state before anything else and counts an untracked
# file as dirty, so staging the notes inside the work tree aborts the publish
# *after* the tag has been pushed (this is what broke v0.2.0). Pin the staging
# location to $RUNNER_TEMP so that regression cannot come back.
grep -q 'notes="\$RUNNER_TEMP/release-notes\.md"' "$release_workflow" ||
	fail "C4: the curated notes must be staged outside the work tree (\$RUNNER_TEMP), or GoReleaser aborts on a dirty git state"
grep -q 'Prepare Release' README.md ||
	fail "C4: README must summarize the explicit Prepare Release gate"

# --- Mutation safety: this checkout must never be touched ------------------
if [ -n "$(git -C "$repo_root" status --porcelain -- CHANGELOG.md .changes 2>/dev/null)" ]; then
	git -C "$repo_root" status --porcelain -- CHANGELOG.md .changes >&2
	fail "this checkout's CHANGELOG.md/.changes were modified by the checks above -- they must only ever touch $work_dir"
fi

echo "release-contract-check: ok (C1 fragment safety, C2 GitHub API contract, C3 changie filename, C4 curated release notes)"
