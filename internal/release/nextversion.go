package release

import (
	"fmt"
	"strings"
)

// PRInfo is the release-relevant metadata for a single merged PR, gathered
// by a workflow via `gh api`. HeadRepo is the head repository's full name
// (e.g. "RafPe/steampipe-plugin-youtrack", matching `gh api`'s
// pull_request.head.repo.full_name); see ValidatePRInput.HeadRepo and
// isGeneratedReleasePR for why it's required.
type PRInfo struct {
	Number     int      `json:"number"`
	Labels     []string `json:"labels"`
	HeadBranch string   `json:"head_branch"`
	HeadRepo   string   `json:"head_repo"`
}

// NextVersionInput models the PRs merged since the previous release.
type NextVersionInput struct {
	PreviousTag string   `json:"previous_tag"`
	PRs         []PRInfo `json:"prs"`
}

// NextVersionResult is the typed result of a next-version computation. It
// is never an error value on its own: Release: false is a normal outcome
// (no releasable PRs), distinct from a validation error in NextVersion's
// second return value.
type NextVersionResult struct {
	Release  bool   `json:"release"`
	Version  string `json:"version,omitempty"`
	Previous string `json:"previous,omitempty"`
	Bump     string `json:"bump,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// bootstrapPreviousVersion is the previous version reported when
// PreviousTag is empty: Changie's own PreviousVersion default for a repo
// with no released versions yet (see task-1-report.md).
var bootstrapPreviousVersion = SemVer{Major: 0, Minor: 0, Patch: 0}

// NextVersion computes the next release version from the PRs merged since
// PreviousTag. Every non-exempt PR must carry exactly one release label
// (validated even when PreviousTag is empty); the highest bump among
// release/major, release/minor, and release/patch PRs wins, independent of
// PR order. Generated release PRs (see isGeneratedReleasePR) are ignored
// entirely: their labels are neither counted nor validated. If no PR is
// releasable (all release/skip, all generated, or the list is empty), the
// result is a typed no-release outcome, not an error.
//
// An empty PreviousTag bootstraps a new repository: the previous version is
// always v0.0.0 and the bump is always forced to "minor" (yielding v0.1.0),
// regardless of the bump labels actually observed on the releasable PRs.
// This only overrides *which version* is computed; it does not force a
// release when there are no releasable PRs.
//
// trustedRepo is passed through to isGeneratedReleasePR for each PR (see
// ValidatePR's trustedRepo parameter for the spoofing rationale): a PR is
// only ignored-as-generated when its HeadRepo equals trustedRepo, its
// HeadBranch is exactly "release/next", and it carries the
// "autorelease: pending" label -- all three, not any one alone.
func NextVersion(input NextVersionInput, trustedRepo string) (NextVersionResult, error) {
	bootstrap := strings.TrimSpace(input.PreviousTag) == ""

	var previous SemVer
	if !bootstrap {
		v, err := ParseSemVer(input.PreviousTag)
		if err != nil {
			return NextVersionResult{}, fmt.Errorf("previous_tag: %w", err)
		}
		previous = v
	}

	highestRank := 0
	highestBump := ""
	releasable := false
	for _, pr := range input.PRs {
		if isGeneratedReleasePR(pr.HeadRepo, pr.HeadBranch, trustedRepo, pr.Labels) {
			continue
		}
		label, err := classifyReleaseLabel(pr.Labels)
		if err != nil {
			return NextVersionResult{}, fmt.Errorf("pr #%d: %w", pr.Number, err)
		}
		if label == labelReleaseSkip {
			continue
		}
		releasable = true
		bump := labelToBump(label)
		if r := bumpRank(bump); r > highestRank {
			highestRank = r
			highestBump = bump
		}
	}

	if !releasable {
		return NextVersionResult{Release: false, Reason: "no releasable prs"}, nil
	}

	if bootstrap {
		previous = bootstrapPreviousVersion
		highestBump = bumpMinor
	}

	next, err := previous.Bump(highestBump)
	if err != nil {
		return NextVersionResult{}, err
	}

	return NextVersionResult{
		Release:  true,
		Version:  next.String(),
		Previous: previous.String(),
		Bump:     highestBump,
	}, nil
}

// labelToBump maps a releasable release/* label to its Bump kind name.
// classifyReleaseLabel restricts label to one of the four release/* labels,
// and callers of labelToBump always exclude labelReleaseSkip beforehand, so
// the default case is unreachable through NextVersion; it is unit tested
// directly for defensiveness.
func labelToBump(label string) string {
	switch label {
	case labelReleaseMajor:
		return bumpMajor
	case labelReleaseMinor:
		return bumpMinor
	case labelReleasePatch:
		return bumpPatch
	default:
		return ""
	}
}

// bumpRank orders bump kinds so the highest-impact bump can be tracked with
// a simple running maximum, independent of input order.
func bumpRank(bump string) int {
	switch bump {
	case bumpMajor:
		return 3
	case bumpMinor:
		return 2
	case bumpPatch:
		return 1
	default:
		return 0
	}
}
