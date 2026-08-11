package release

import (
	"fmt"
)

// Release labels that classify a PR's intent. Exactly one must be present
// on any PR that is not a generated release PR.
const (
	labelReleaseMajor = "release/major"
	labelReleaseMinor = "release/minor"
	labelReleasePatch = "release/patch"
	labelReleaseSkip  = "release/skip"

	// labelAutoreleasePending marks a bot-generated release PR.
	labelAutoreleasePending = "autorelease: pending"
	// generatedReleaseBranch is the head branch Task 4's workflow opens
	// generated release PRs from.
	generatedReleaseBranch = "release/next"
)

var releaseLabels = []string{labelReleaseMajor, labelReleaseMinor, labelReleasePatch, labelReleaseSkip}

// ValidatePRInput models the PR metadata releasectl validate-pr consumes,
// gathered by a workflow via `gh api` (never fetched by this package).
// HeadRepo is the head repository's full name (e.g.
// "RafPe/steampipe-youtrack", matching `gh api`'s
// pull_request.head.repo.full_name) and is required so the generated-release
// exemption cannot be spoofed by a fork (see isGeneratedReleasePR).
type ValidatePRInput struct {
	Labels       []string `json:"labels"`
	HeadBranch   string   `json:"head_branch"`
	HeadRepo     string   `json:"head_repo"`
	ChangedFiles []string `json:"changed_files"`
}

// ValidatePR checks a PR's release-label and changelog-fragment metadata
// against repository policy. fragmentsDir is the changelog fragments
// directory, both as a repo-relative path (matched against
// input.ChangedFiles) and as a filesystem path (read from the process's
// working directory, i.e. the repo root in CI). trustedRepo is the base
// repository's full name (e.g. "RafPe/steampipe-youtrack"); only a PR whose
// HeadRepo matches it can qualify for the generated-release exemption (see
// isGeneratedReleasePR) -- callers must supply a non-empty trustedRepo, or
// no PR will ever be treated as exempt. On success it returns a
// human-readable summary suitable for printing; on failure it returns an
// error describing the violation.
func ValidatePR(input ValidatePRInput, fragmentsDir, trustedRepo string) (string, error) {
	if isGeneratedReleasePR(input.HeadRepo, input.HeadBranch, trustedRepo, input.Labels) {
		return "ok: generated release pr, exempt from label and fragment checks", nil
	}

	label, err := classifyReleaseLabel(input.Labels)
	if err != nil {
		return "", err
	}

	fragments, err := fragmentCandidates(fragmentsDir, input.ChangedFiles)
	if err != nil {
		return "", err
	}

	if label == labelReleaseSkip {
		if len(fragments) > 0 {
			return "", fmt.Errorf("release/skip must not include changelog fragments, found %d", len(fragments))
		}
		return "ok: release/skip, no changelog fragments", nil
	}

	if len(fragments) == 0 {
		return "", fmt.Errorf("%s requires at least one changelog fragment under %s", label, fragmentsDir)
	}

	for _, path := range fragments {
		if err := validateFragmentFile(fragmentsDir, path); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("ok: %s with %d changelog fragment(s)", label, len(fragments)), nil
}

// isGeneratedReleasePR reports whether a PR is the bot-generated release PR
// that batches pending fragments into a version, and is therefore exempt
// from label and fragment checks.
//
// All three signals must hold, not any one alone: headRepo must equal
// trustedRepo, headBranch must be exactly "release/next", and the
// "autorelease: pending" label must be present. Any partial combination
// (right branch from a fork, right label on an ordinary branch, right repo
// without the branch or label, and so on) is not exempt and is validated as
// an ordinary PR. Branch name and label are both attacker-controlled by
// anyone who can open a PR (including from a fork), so neither alone -- nor
// both together -- is sufficient; a fork could otherwise name its branch
// "release/next" and carry the label to bypass all metadata validation. An
// empty trustedRepo never matches (fails closed): callers must supply the
// real trusted base repository explicitly, never rely on a default.
func isGeneratedReleasePR(headRepo, headBranch, trustedRepo string, labels []string) bool {
	if trustedRepo == "" || headRepo != trustedRepo {
		return false
	}
	if headBranch != generatedReleaseBranch {
		return false
	}
	for _, l := range labels {
		if l == labelAutoreleasePending {
			return true
		}
	}
	return false
}

// classifyReleaseLabel finds the single release/* label among labels.
// Duplicates of the same label count once. Zero or multiple distinct
// release labels is an error.
func classifyReleaseLabel(labels []string) (string, error) {
	seen := make(map[string]bool, len(releaseLabels))
	var matched []string
	for _, l := range labels {
		for _, rl := range releaseLabels {
			if l == rl && !seen[rl] {
				seen[rl] = true
				matched = append(matched, rl)
			}
		}
	}
	switch len(matched) {
	case 0:
		return "", fmt.Errorf("no release label found; expected exactly one of %s, %s, %s, %s",
			labelReleaseMajor, labelReleaseMinor, labelReleasePatch, labelReleaseSkip)
	case 1:
		return matched[0], nil
	default:
		return "", fmt.Errorf("multiple release labels found, expected exactly one: %v", matched)
	}
}
