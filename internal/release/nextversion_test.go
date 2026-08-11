package release

import (
	"strings"
	"testing"
)

func TestNextVersion(t *testing.T) {
	t.Parallel()

	pendingLabel := []string{"autorelease: pending"}

	tests := map[string]struct {
		input       NextVersionInput
		trustedRepo string
		want        NextVersionResult
		wantErr     string
	}{
		"empty prs list": {
			input:       NextVersionInput{PreviousTag: "v0.1.0"},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: false, Reason: "no releasable prs"},
		},
		"only-skip prs": {
			input: NextVersionInput{
				PreviousTag: "v0.1.0",
				PRs: []PRInfo{
					{Number: 1, Labels: []string{"release/skip"}, HeadBranch: "chore/a", HeadRepo: trustedRepoForTests},
					{Number: 2, Labels: []string{"release/skip"}, HeadBranch: "chore/b", HeadRepo: trustedRepoForTests},
				},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: false, Reason: "no releasable prs"},
		},
		"patch bump": {
			input: NextVersionInput{
				PreviousTag: "v0.1.0",
				PRs:         []PRInfo{{Number: 1, Labels: []string{"release/patch"}, HeadBranch: "fix/a", HeadRepo: trustedRepoForTests}},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: true, Version: "v0.1.1", Previous: "v0.1.0", Bump: "patch"},
		},
		"minor bump": {
			input: NextVersionInput{
				PreviousTag: "v0.1.0",
				PRs:         []PRInfo{{Number: 12, Labels: []string{"release/minor"}, HeadBranch: "feat/x", HeadRepo: trustedRepoForTests}},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: true, Version: "v0.2.0", Previous: "v0.1.0", Bump: "minor"},
		},
		"major bump": {
			input: NextVersionInput{
				PreviousTag: "v0.1.0",
				PRs: []PRInfo{
					{Number: 1, Labels: []string{"release/major"}, HeadBranch: "feat/breaking", HeadRepo: trustedRepoForTests},
				},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: true, Version: "v1.0.0", Previous: "v0.1.0", Bump: "major"},
		},
		"pre-1.0 standard semver bump rules": {
			input: NextVersionInput{
				PreviousTag: "v0.1.0",
				PRs: []PRInfo{
					{Number: 1, Labels: []string{"release/major"}, HeadBranch: "feat/breaking", HeadRepo: trustedRepoForTests},
				},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: true, Version: "v1.0.0", Previous: "v0.1.0", Bump: "major"},
		},
		"highest bump wins regardless of order, minor then major": {
			input: NextVersionInput{
				PreviousTag: "v1.0.0",
				PRs: []PRInfo{
					{Number: 1, Labels: []string{"release/minor"}, HeadBranch: "feat/x", HeadRepo: trustedRepoForTests},
					{Number: 2, Labels: []string{"release/major"}, HeadBranch: "feat/breaking", HeadRepo: trustedRepoForTests},
				},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: true, Version: "v2.0.0", Previous: "v1.0.0", Bump: "major"},
		},
		"highest bump wins regardless of order, major then minor": {
			input: NextVersionInput{
				PreviousTag: "v1.0.0",
				PRs: []PRInfo{
					{Number: 2, Labels: []string{"release/major"}, HeadBranch: "feat/breaking", HeadRepo: trustedRepoForTests},
					{Number: 1, Labels: []string{"release/minor"}, HeadBranch: "feat/x", HeadRepo: trustedRepoForTests},
				},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: true, Version: "v2.0.0", Previous: "v1.0.0", Bump: "major"},
		},
		"highest bump wins among patch, minor, skip": {
			input: NextVersionInput{
				PreviousTag: "v1.0.0",
				PRs: []PRInfo{
					{Number: 1, Labels: []string{"release/patch"}, HeadBranch: "fix/a", HeadRepo: trustedRepoForTests},
					{Number: 2, Labels: []string{"release/skip"}, HeadBranch: "chore/b", HeadRepo: trustedRepoForTests},
					{Number: 3, Labels: []string{"release/minor"}, HeadBranch: "feat/c", HeadRepo: trustedRepoForTests},
				},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: true, Version: "v1.1.0", Previous: "v1.0.0", Bump: "minor"},
		},
		"generated pr is ignored entirely, including unvalidated labels, only when all three signals align": {
			input: NextVersionInput{
				PreviousTag: "v1.0.0",
				PRs: []PRInfo{
					{Number: 1, Labels: []string{"release/patch"}, HeadBranch: "fix/a", HeadRepo: trustedRepoForTests},
					{
						Number: 2, Labels: append([]string{"release/major", "release/minor"}, pendingLabel...),
						HeadBranch: "release/next", HeadRepo: trustedRepoForTests,
					},
				},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: true, Version: "v1.0.1", Previous: "v1.0.0", Bump: "patch"},
		},
		"only a generated pr yields no release": {
			input: NextVersionInput{
				PreviousTag: "v1.0.0",
				PRs: []PRInfo{
					{Number: 1, HeadBranch: "release/next", HeadRepo: trustedRepoForTests, Labels: pendingLabel},
				},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: false, Reason: "no releasable prs"},
		},
		"fork pr spoofing branch and label is validated normally and errors": {
			input: NextVersionInput{
				PreviousTag: "v1.0.0",
				PRs: []PRInfo{
					{
						Number: 13, Labels: append([]string{"release/major", "release/minor"}, pendingLabel...),
						HeadBranch: "release/next", HeadRepo: forkRepoForTests,
					},
				},
			},
			trustedRepo: trustedRepoForTests,
			wantErr:     "pr #13",
		},
		"trusted repo pr on release/next without the label is validated normally and errors": {
			input: NextVersionInput{
				PreviousTag: "v1.0.0",
				PRs: []PRInfo{
					{Number: 21, Labels: []string{"bug"}, HeadBranch: "release/next", HeadRepo: trustedRepoForTests},
				},
			},
			trustedRepo: trustedRepoForTests,
			wantErr:     "pr #21",
		},
		"pr with zero release labels errors naming the pr number": {
			input: NextVersionInput{
				PreviousTag: "v1.0.0",
				PRs:         []PRInfo{{Number: 42, Labels: []string{"bug"}, HeadBranch: "fix/a", HeadRepo: trustedRepoForTests}},
			},
			trustedRepo: trustedRepoForTests,
			wantErr:     "pr #42",
		},
		"pr with multiple release labels errors naming the pr number": {
			input: NextVersionInput{
				PreviousTag: "v1.0.0",
				PRs: []PRInfo{
					{Number: 7, Labels: []string{"release/major", "release/patch"}, HeadBranch: "feat/x", HeadRepo: trustedRepoForTests},
				},
			},
			trustedRepo: trustedRepoForTests,
			wantErr:     "pr #7",
		},
		"malformed previous_tag errors": {
			input: NextVersionInput{
				PreviousTag: "not-a-version",
				PRs:         []PRInfo{{Number: 1, Labels: []string{"release/patch"}, HeadBranch: "fix/a", HeadRepo: trustedRepoForTests}},
			},
			trustedRepo: trustedRepoForTests,
			wantErr:     "previous_tag",
		},
		"previous_tag with prerelease suffix errors": {
			input: NextVersionInput{
				PreviousTag: "v1.0.0-rc1",
				PRs:         []PRInfo{{Number: 1, Labels: []string{"release/patch"}, HeadBranch: "fix/a", HeadRepo: trustedRepoForTests}},
			},
			trustedRepo: trustedRepoForTests,
			wantErr:     "previous_tag",
		},
		"previous_tag overflow errors": {
			input: NextVersionInput{
				PreviousTag: "v99999999999.0.0",
				PRs:         []PRInfo{{Number: 1, Labels: []string{"release/patch"}, HeadBranch: "fix/a", HeadRepo: trustedRepoForTests}},
			},
			trustedRepo: trustedRepoForTests,
			wantErr:     "previous_tag",
		},
		"bootstrap forces v0.1.0 regardless of label mix, still validates labels": {
			input: NextVersionInput{
				PreviousTag: "",
				PRs:         []PRInfo{{Number: 1, Labels: []string{"release/patch"}, HeadBranch: "fix/a", HeadRepo: trustedRepoForTests}},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: true, Version: "v0.1.0", Previous: "v0.0.0", Bump: "minor"},
		},
		"bootstrap with a major-bump pr still yields v0.1.0": {
			input: NextVersionInput{
				PreviousTag: "  ",
				PRs:         []PRInfo{{Number: 1, Labels: []string{"release/major"}, HeadBranch: "feat/breaking", HeadRepo: trustedRepoForTests}},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: true, Version: "v0.1.0", Previous: "v0.0.0", Bump: "minor"},
		},
		"bootstrap still validates pr labels": {
			input: NextVersionInput{
				PreviousTag: "",
				PRs: []PRInfo{
					{Number: 9, Labels: []string{"release/major", "release/minor"}, HeadBranch: "feat/x", HeadRepo: trustedRepoForTests},
				},
			},
			trustedRepo: trustedRepoForTests,
			wantErr:     "pr #9",
		},
		"bootstrap with only-skip prs is still no release": {
			input: NextVersionInput{
				PreviousTag: "",
				PRs:         []PRInfo{{Number: 1, Labels: []string{"release/skip"}, HeadBranch: "chore/a", HeadRepo: trustedRepoForTests}},
			},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: false, Reason: "no releasable prs"},
		},
		"bump overflow propagates as an error": {
			input: NextVersionInput{
				PreviousTag: "v2147483647.0.0",
				PRs:         []PRInfo{{Number: 1, Labels: []string{"release/major"}, HeadBranch: "feat/breaking", HeadRepo: trustedRepoForTests}},
			},
			trustedRepo: trustedRepoForTests,
			wantErr:     "range",
		},
		"bootstrap with empty prs is still no release": {
			input:       NextVersionInput{PreviousTag: ""},
			trustedRepo: trustedRepoForTests,
			want:        NextVersionResult{Release: false, Reason: "no releasable prs"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := NextVersion(tt.input, tt.trustedRepo)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("NextVersion() error = %v, want nil", err)
				}
				if got != tt.want {
					t.Errorf("NextVersion() = %+v, want %+v", got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("NextVersion() = %+v, want error containing %q", got, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("NextVersion() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

// TestNextVersionDeterministic exercises determinism across repeated runs
// and across permutations of PR order (brief requirement: "deterministic
// across repeated runs and input orderings").
func TestNextVersionDeterministic(t *testing.T) {
	t.Parallel()

	orderings := [][]PRInfo{
		{
			{Number: 1, Labels: []string{"release/patch"}, HeadBranch: "fix/a", HeadRepo: trustedRepoForTests},
			{Number: 2, Labels: []string{"release/major"}, HeadBranch: "feat/breaking", HeadRepo: trustedRepoForTests},
			{Number: 3, Labels: []string{"release/minor"}, HeadBranch: "feat/c", HeadRepo: trustedRepoForTests},
		},
		{
			{Number: 3, Labels: []string{"release/minor"}, HeadBranch: "feat/c", HeadRepo: trustedRepoForTests},
			{Number: 1, Labels: []string{"release/patch"}, HeadBranch: "fix/a", HeadRepo: trustedRepoForTests},
			{Number: 2, Labels: []string{"release/major"}, HeadBranch: "feat/breaking", HeadRepo: trustedRepoForTests},
		},
		{
			{Number: 2, Labels: []string{"release/major"}, HeadBranch: "feat/breaking", HeadRepo: trustedRepoForTests},
			{Number: 3, Labels: []string{"release/minor"}, HeadBranch: "feat/c", HeadRepo: trustedRepoForTests},
			{Number: 1, Labels: []string{"release/patch"}, HeadBranch: "fix/a", HeadRepo: trustedRepoForTests},
		},
	}

	var want NextVersionResult
	for i, prs := range orderings {
		for run := 0; run < 2; run++ {
			got, err := NextVersion(NextVersionInput{PreviousTag: "v1.0.0", PRs: prs}, trustedRepoForTests)
			if err != nil {
				t.Fatalf("NextVersion() error = %v, want nil", err)
			}
			if i == 0 && run == 0 {
				want = got
				continue
			}
			if got != want {
				t.Errorf("NextVersion() ordering %d run %d = %+v, want %+v (determinism violated)", i, run, got, want)
			}
		}
	}
}

func TestLabelToBump(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		label string
		want  string
	}{
		"major":   {label: labelReleaseMajor, want: bumpMajor},
		"minor":   {label: labelReleaseMinor, want: bumpMinor},
		"patch":   {label: labelReleasePatch, want: bumpPatch},
		"unknown": {label: "bogus", want: ""},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := labelToBump(tt.label); got != tt.want {
				t.Errorf("labelToBump(%q) = %q, want %q", tt.label, got, tt.want)
			}
		})
	}
}

func TestBumpRank(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		bump string
		want int
	}{
		"major":   {bump: bumpMajor, want: 3},
		"minor":   {bump: bumpMinor, want: 2},
		"patch":   {bump: bumpPatch, want: 1},
		"unknown": {bump: "bogus", want: 0},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := bumpRank(tt.bump); got != tt.want {
				t.Errorf("bumpRank(%q) = %d, want %d", tt.bump, got, tt.want)
			}
		})
	}
}
