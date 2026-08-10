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
type ValidatePRInput struct {
	Labels       []string `json:"labels"`
	HeadBranch   string   `json:"head_branch"`
	ChangedFiles []string `json:"changed_files"`
}

// ValidatePR checks a PR's release-label and changelog-fragment metadata
// against repository policy. fragmentsDir is the changelog fragments
// directory, both as a repo-relative path (matched against
// input.ChangedFiles) and as a filesystem path (read from the process's
// working directory, i.e. the repo root in CI). On success it returns a
// human-readable summary suitable for printing; on failure it returns an
// error describing the violation.
func ValidatePR(input ValidatePRInput, fragmentsDir string) (string, error) {
	if isGeneratedReleasePR(input.HeadBranch, input.Labels) {
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
// that batches pending fragments into a version: identified by head branch
// or the "autorelease: pending" label. Generated release PRs are exempt
// from label and fragment checks.
func isGeneratedReleasePR(headBranch string, labels []string) bool {
	if headBranch == generatedReleaseBranch {
		return true
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
