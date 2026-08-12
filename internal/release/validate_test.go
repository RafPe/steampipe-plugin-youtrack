package release

import (
	"os"
	"strings"
	"testing"
)

// trustedRepoForTests is the base repository full name used across
// internal/release's tests wherever a generated-release exemption should
// actually be granted (see isGeneratedReleasePR).
const trustedRepoForTests = "RafPe/steampipe-plugin-youtrack"

// forkRepoForTests is an untrusted head repository, used to prove a fork
// cannot spoof the generated-release exemption.
const forkRepoForTests = "attacker/steampipe-plugin-youtrack"

func TestClassifyReleaseLabel(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		labels  []string
		want    string
		wantErr string
	}{
		"major":                 {labels: []string{"release/major"}, want: labelReleaseMajor},
		"minor":                 {labels: []string{"release/minor"}, want: labelReleaseMinor},
		"patch":                 {labels: []string{"release/patch"}, want: labelReleasePatch},
		"skip":                  {labels: []string{"release/skip"}, want: labelReleaseSkip},
		"amid unrelated labels": {labels: []string{"bug", "release/patch", "needs-review"}, want: labelReleasePatch},
		"duplicate label":       {labels: []string{"release/patch", "release/patch"}, want: labelReleasePatch},
		"no labels":             {labels: nil, wantErr: "no release label found"},
		"unrelated labels only": {labels: []string{"bug", "needs-review"}, wantErr: "no release label found"},
		"two release labels":    {labels: []string{"release/patch", "release/minor"}, wantErr: "multiple release labels found"},
		"all four release labels": {
			labels:  []string{"release/major", "release/minor", "release/patch", "release/skip"},
			wantErr: "multiple release labels found",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := classifyReleaseLabel(tt.labels)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("classifyReleaseLabel(%v) error = %v, want nil", tt.labels, err)
				}
				if got != tt.want {
					t.Errorf("classifyReleaseLabel(%v) = %q, want %q", tt.labels, got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("classifyReleaseLabel(%v) = %q, want error containing %q", tt.labels, got, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("classifyReleaseLabel(%v) error = %v, want containing %q", tt.labels, err, tt.wantErr)
			}
		})
	}
}

// TestIsGeneratedReleasePRSpoofMatrix exhaustively covers all 2x2x2
// combinations of the three exemption signals (head repo trusted or not,
// head branch exactly "release/next" or not, "autorelease: pending" label
// present or not): only the all-true combination is exempt. This is the
// direct regression test for the spoofing blocker -- a fork that controls
// its own branch name and labels must never be able to grant itself the
// exemption by matching only one or two of the three signals.
func TestIsGeneratedReleasePRSpoofMatrix(t *testing.T) {
	t.Parallel()

	pendingLabel := []string{"autorelease: pending"}

	tests := map[string]struct {
		headRepo   string
		headBranch string
		labels     []string
		want       bool
	}{
		"all three true is exempt": {
			headRepo: trustedRepoForTests, headBranch: "release/next", labels: pendingLabel, want: true,
		},
		"fork repo, right branch, right label": {
			headRepo: forkRepoForTests, headBranch: "release/next", labels: pendingLabel, want: false,
		},
		"trusted repo, wrong branch, right label": {
			headRepo: trustedRepoForTests, headBranch: "feat/x", labels: pendingLabel, want: false,
		},
		"trusted repo, right branch, missing label": {
			headRepo: trustedRepoForTests, headBranch: "release/next", labels: nil, want: false,
		},
		"trusted repo only": {
			headRepo: trustedRepoForTests, headBranch: "feat/x", labels: nil, want: false,
		},
		"right branch only": {
			headRepo: forkRepoForTests, headBranch: "release/next", labels: nil, want: false,
		},
		"right label only": {
			headRepo: forkRepoForTests, headBranch: "feat/x", labels: pendingLabel, want: false,
		},
		"none true": {
			headRepo: forkRepoForTests, headBranch: "feat/x", labels: nil, want: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := isGeneratedReleasePR(tt.headRepo, tt.headBranch, trustedRepoForTests, tt.labels)
			if got != tt.want {
				t.Errorf("isGeneratedReleasePR(%q, %q, %q, %v) = %v, want %v",
					tt.headRepo, tt.headBranch, trustedRepoForTests, tt.labels, got, tt.want)
			}
		})
	}
}

func TestIsGeneratedReleasePR(t *testing.T) {
	t.Parallel()

	pendingLabel := []string{"autorelease: pending"}

	tests := map[string]struct {
		headRepo    string
		headBranch  string
		labels      []string
		trustedRepo string
		want        bool
	}{
		"all signals aligned": {
			headRepo: trustedRepoForTests, headBranch: "release/next", labels: pendingLabel,
			trustedRepo: trustedRepoForTests, want: true,
		},
		"similarly named label does not count": {
			headRepo: trustedRepoForTests, headBranch: "release/next", labels: []string{"autorelease: complete"},
			trustedRepo: trustedRepoForTests, want: false,
		},
		"similarly named branch does not count": {
			headRepo: trustedRepoForTests, headBranch: "release/next-2", labels: pendingLabel,
			trustedRepo: trustedRepoForTests, want: false,
		},
		"empty trusted repo never matches, even an empty head repo": {
			headRepo: "", headBranch: "release/next", labels: pendingLabel,
			trustedRepo: "", want: false,
		},
		"nothing set at all": {
			trustedRepo: trustedRepoForTests, want: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := isGeneratedReleasePR(tt.headRepo, tt.headBranch, tt.trustedRepo, tt.labels)
			if got != tt.want {
				t.Errorf("isGeneratedReleasePR(%q, %q, %q, %v) = %v, want %v",
					tt.headRepo, tt.headBranch, tt.trustedRepo, tt.labels, got, tt.want)
			}
		})
	}
}

func TestValidatePR(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input        ValidatePRInput
		fragmentsDir string
		trustedRepo  string
		wantMsg      string
		wantErr      string
	}{
		"generated release pr is exempt when all three signals align": {
			input: ValidatePRInput{
				HeadRepo:   trustedRepoForTests,
				HeadBranch: "release/next",
				Labels:     []string{"autorelease: pending"},
				// No other release label and no fragments: would otherwise
				// fail every check, proving exemption short-circuits them.
			},
			fragmentsDir: "testdata/fragments/valid",
			trustedRepo:  trustedRepoForTests,
			wantMsg:      "ok: generated release pr, exempt from label and fragment checks",
		},
		"fork PR with spoofed branch and label is not exempt and is validated normally": {
			input: ValidatePRInput{
				HeadRepo:   forkRepoForTests,
				HeadBranch: "release/next",
				Labels:     []string{"autorelease: pending", "release/major", "release/minor"},
			},
			fragmentsDir: "testdata/fragments/valid",
			trustedRepo:  trustedRepoForTests,
			wantErr:      "multiple release labels found",
		},
		"trusted repo with only the branch signal is not exempt": {
			input: ValidatePRInput{
				HeadRepo:   trustedRepoForTests,
				HeadBranch: "release/next",
				Labels:     []string{"bug"},
			},
			fragmentsDir: "testdata/fragments/valid",
			trustedRepo:  trustedRepoForTests,
			wantErr:      "no release label found",
		},
		"zero release labels errors": {
			input:        ValidatePRInput{HeadRepo: trustedRepoForTests, HeadBranch: "feat/x", Labels: []string{"bug"}},
			fragmentsDir: "testdata/fragments/valid",
			trustedRepo:  trustedRepoForTests,
			wantErr:      "no release label found",
		},
		"multiple release labels errors": {
			input: ValidatePRInput{
				HeadRepo: trustedRepoForTests, HeadBranch: "feat/x",
				Labels: []string{"release/major", "release/patch"},
			},
			fragmentsDir: "testdata/fragments/valid",
			trustedRepo:  trustedRepoForTests,
			wantErr:      "multiple release labels found",
		},
		"release/skip with no fragments succeeds": {
			input: ValidatePRInput{
				HeadRepo:     trustedRepoForTests,
				HeadBranch:   "chore/x",
				Labels:       []string{"release/skip"},
				ChangedFiles: []string{"README.md"},
			},
			fragmentsDir: "testdata/fragments/valid",
			trustedRepo:  trustedRepoForTests,
			wantMsg:      "ok: release/skip, no changelog fragments",
		},
		"release/skip with a fragment errors": {
			input: ValidatePRInput{
				HeadRepo:     trustedRepoForTests,
				HeadBranch:   "chore/x",
				Labels:       []string{"release/skip"},
				ChangedFiles: []string{"testdata/fragments/valid/Added-1.yaml"},
			},
			fragmentsDir: "testdata/fragments/valid",
			trustedRepo:  trustedRepoForTests,
			wantErr:      "must not include changelog fragments",
		},
		"releasable label with no fragment errors": {
			input: ValidatePRInput{
				HeadRepo:     trustedRepoForTests,
				HeadBranch:   "feat/x",
				Labels:       []string{"release/patch"},
				ChangedFiles: []string{"youtrack/plugin.go"},
			},
			fragmentsDir: "testdata/fragments/valid",
			trustedRepo:  trustedRepoForTests,
			wantErr:      "requires at least one changelog fragment",
		},
		"releasable label with one valid fragment succeeds": {
			input: ValidatePRInput{
				HeadRepo:     trustedRepoForTests,
				HeadBranch:   "feat/x",
				Labels:       []string{"release/patch"},
				ChangedFiles: []string{"testdata/fragments/valid/Added-1.yaml", "youtrack/plugin.go"},
			},
			fragmentsDir: "testdata/fragments/valid",
			trustedRepo:  trustedRepoForTests,
			wantMsg:      "ok: release/patch with 1 changelog fragment(s)",
		},
		"releasable label with multiple valid fragments succeeds": {
			input: ValidatePRInput{
				HeadRepo:   trustedRepoForTests,
				HeadBranch: "feat/x",
				Labels:     []string{"release/minor"},
				ChangedFiles: []string{
					"testdata/fragments/valid/Added-1.yaml",
					"testdata/fragments/valid/Changed-1.yaml",
				},
			},
			fragmentsDir: "testdata/fragments/valid",
			trustedRepo:  trustedRepoForTests,
			wantMsg:      "ok: release/minor with 2 changelog fragment(s)",
		},
		"malformed fragment yaml errors": {
			input: ValidatePRInput{
				HeadRepo:     trustedRepoForTests,
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"testdata/fragments/bad/malformed.yaml"},
			},
			fragmentsDir: "testdata/fragments/bad",
			trustedRepo:  trustedRepoForTests,
			wantErr:      "invalid yaml",
		},
		"unknown kind fragment errors": {
			input: ValidatePRInput{
				HeadRepo:     trustedRepoForTests,
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"testdata/fragments/bad/unknown-kind.yaml"},
			},
			fragmentsDir: "testdata/fragments/bad",
			trustedRepo:  trustedRepoForTests,
			wantErr:      "unknown kind",
		},
		"empty body fragment errors": {
			input: ValidatePRInput{
				HeadRepo:     trustedRepoForTests,
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"testdata/fragments/bad/empty-body.yaml"},
			},
			fragmentsDir: "testdata/fragments/bad",
			trustedRepo:  trustedRepoForTests,
			wantErr:      "empty body",
		},
		"escaping fragment path errors": {
			input: ValidatePRInput{
				HeadRepo:     trustedRepoForTests,
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"testdata/fragments/valid/../../../etc/passwd"},
			},
			fragmentsDir: "testdata/fragments/valid",
			trustedRepo:  trustedRepoForTests,
			wantErr:      "escapes",
		},
		"absolute changed file path errors": {
			input: ValidatePRInput{
				HeadRepo:     trustedRepoForTests,
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"/etc/passwd"},
			},
			fragmentsDir: "testdata/fragments/valid",
			trustedRepo:  trustedRepoForTests,
			wantErr:      "repo-relative",
		},
		"symlink escaping fragment errors": {
			input: ValidatePRInput{
				HeadRepo:     trustedRepoForTests,
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"testdata/fragments/symlinks/escape.yaml"},
			},
			fragmentsDir: "testdata/fragments/symlinks",
			trustedRepo:  trustedRepoForTests,
			wantErr:      "escapes",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidatePR(tt.input, tt.fragmentsDir, tt.trustedRepo)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidatePR() error = %v, want nil", err)
				}
				if got != tt.wantMsg {
					t.Errorf("ValidatePR() = %q, want %q", got, tt.wantMsg)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidatePR() = %q, want error containing %q", got, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ValidatePR() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

// TestValidatePRRejectsCredentialFragment exercises the credential-shape
// rejection through the full ValidatePR path (fragmentCandidates ->
// validateFragmentFile), not just validateFragmentFile directly. The
// fixture is generated at runtime (see fakeCredentialShape in
// fragments_test.go) rather than committed, so no realistic-looking token
// literal exists anywhere in this repository's history.
//
// ChangedFiles entries (and therefore fragmentsDir, for a match to be
// possible at all) must be repo-relative -- t.TempDir() is absolute, so a
// relative scratch directory is created under testdata/ instead and
// removed afterward; nothing here is ever committed.
func TestValidatePRRejectsCredentialFragment(t *testing.T) {
	t.Parallel()

	dir, err := os.MkdirTemp("testdata", "credential-fixture-*")
	if err != nil {
		t.Fatalf("create scratch fragments dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	token := fakeCredentialShape("ghp")
	fragmentPath := writeFragmentFile(t, dir, "Added-1.yaml", kindAdded, "Rotate token "+token+" please.")

	input := ValidatePRInput{
		HeadRepo:     trustedRepoForTests,
		HeadBranch:   "feat/x",
		Labels:       []string{"release/major"},
		ChangedFiles: []string{fragmentPath},
	}

	_, err = ValidatePR(input, dir, trustedRepoForTests)
	if err == nil {
		t.Fatal("ValidatePR() = nil, want an error for a credential-shaped fragment")
	}
	if !strings.Contains(err.Error(), "credential") {
		t.Errorf("ValidatePR() error = %v, want containing %q", err, "credential")
	}
	if strings.Contains(err.Error(), token) {
		t.Errorf("ValidatePR() error echoed the credential: %v", err)
	}
}
