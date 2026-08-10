package release

import (
	"strings"
	"testing"
)

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

func TestIsGeneratedReleasePR(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		headBranch string
		labels     []string
		want       bool
	}{
		"generated branch":          {headBranch: "release/next", want: true},
		"autorelease pending label": {headBranch: "feat/x", labels: []string{"autorelease: pending"}, want: true},
		"both signals":              {headBranch: "release/next", labels: []string{"autorelease: pending"}, want: true},
		"ordinary feature branch":   {headBranch: "feat/x", labels: []string{"release/patch"}, want: false},
		"similarly named label":     {headBranch: "feat/x", labels: []string{"autorelease: complete"}, want: false},
		"similarly named branch":    {headBranch: "release/next-2", labels: nil, want: false},
		"no branch or labels":       {want: false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := isGeneratedReleasePR(tt.headBranch, tt.labels)
			if got != tt.want {
				t.Errorf("isGeneratedReleasePR(%q, %v) = %v, want %v", tt.headBranch, tt.labels, got, tt.want)
			}
		})
	}
}

func TestValidatePR(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input        ValidatePRInput
		fragmentsDir string
		wantMsg      string
		wantErr      string
	}{
		"generated release pr via head branch is exempt": {
			input: ValidatePRInput{
				HeadBranch: "release/next",
				// No release label and no fragments: would otherwise fail
				// every check, proving exemption short-circuits them.
			},
			fragmentsDir: "testdata/fragments/valid",
			wantMsg:      "ok: generated release pr, exempt from label and fragment checks",
		},
		"generated release pr via label is exempt": {
			input: ValidatePRInput{
				HeadBranch: "chore/bump",
				Labels:     []string{"autorelease: pending", "release/major", "release/minor"},
			},
			fragmentsDir: "testdata/fragments/valid",
			wantMsg:      "ok: generated release pr, exempt from label and fragment checks",
		},
		"zero release labels errors": {
			input:        ValidatePRInput{HeadBranch: "feat/x", Labels: []string{"bug"}},
			fragmentsDir: "testdata/fragments/valid",
			wantErr:      "no release label found",
		},
		"multiple release labels errors": {
			input:        ValidatePRInput{HeadBranch: "feat/x", Labels: []string{"release/major", "release/patch"}},
			fragmentsDir: "testdata/fragments/valid",
			wantErr:      "multiple release labels found",
		},
		"release/skip with no fragments succeeds": {
			input: ValidatePRInput{
				HeadBranch:   "chore/x",
				Labels:       []string{"release/skip"},
				ChangedFiles: []string{"README.md"},
			},
			fragmentsDir: "testdata/fragments/valid",
			wantMsg:      "ok: release/skip, no changelog fragments",
		},
		"release/skip with a fragment errors": {
			input: ValidatePRInput{
				HeadBranch:   "chore/x",
				Labels:       []string{"release/skip"},
				ChangedFiles: []string{"testdata/fragments/valid/Added-1.yaml"},
			},
			fragmentsDir: "testdata/fragments/valid",
			wantErr:      "must not include changelog fragments",
		},
		"releasable label with no fragment errors": {
			input: ValidatePRInput{
				HeadBranch:   "feat/x",
				Labels:       []string{"release/patch"},
				ChangedFiles: []string{"youtrack/plugin.go"},
			},
			fragmentsDir: "testdata/fragments/valid",
			wantErr:      "requires at least one changelog fragment",
		},
		"releasable label with one valid fragment succeeds": {
			input: ValidatePRInput{
				HeadBranch:   "feat/x",
				Labels:       []string{"release/patch"},
				ChangedFiles: []string{"testdata/fragments/valid/Added-1.yaml", "youtrack/plugin.go"},
			},
			fragmentsDir: "testdata/fragments/valid",
			wantMsg:      "ok: release/patch with 1 changelog fragment(s)",
		},
		"releasable label with multiple valid fragments succeeds": {
			input: ValidatePRInput{
				HeadBranch: "feat/x",
				Labels:     []string{"release/minor"},
				ChangedFiles: []string{
					"testdata/fragments/valid/Added-1.yaml",
					"testdata/fragments/valid/Changed-1.yaml",
				},
			},
			fragmentsDir: "testdata/fragments/valid",
			wantMsg:      "ok: release/minor with 2 changelog fragment(s)",
		},
		"malformed fragment yaml errors": {
			input: ValidatePRInput{
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"testdata/fragments/bad/malformed.yaml"},
			},
			fragmentsDir: "testdata/fragments/bad",
			wantErr:      "invalid yaml",
		},
		"unknown kind fragment errors": {
			input: ValidatePRInput{
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"testdata/fragments/bad/unknown-kind.yaml"},
			},
			fragmentsDir: "testdata/fragments/bad",
			wantErr:      "unknown kind",
		},
		"empty body fragment errors": {
			input: ValidatePRInput{
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"testdata/fragments/bad/empty-body.yaml"},
			},
			fragmentsDir: "testdata/fragments/bad",
			wantErr:      "empty body",
		},
		"credential fragment errors": {
			input: ValidatePRInput{
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"testdata/fragments/bad/credential-ghp.yaml"},
			},
			fragmentsDir: "testdata/fragments/bad",
			wantErr:      "credential",
		},
		"escaping fragment path errors": {
			input: ValidatePRInput{
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"testdata/fragments/valid/../../../etc/passwd"},
			},
			fragmentsDir: "testdata/fragments/valid",
			wantErr:      "escapes",
		},
		"absolute changed file path errors": {
			input: ValidatePRInput{
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"/etc/passwd"},
			},
			fragmentsDir: "testdata/fragments/valid",
			wantErr:      "repo-relative",
		},
		"symlink escaping fragment errors": {
			input: ValidatePRInput{
				HeadBranch:   "feat/x",
				Labels:       []string{"release/major"},
				ChangedFiles: []string{"testdata/fragments/symlinks/escape.yaml"},
			},
			fragmentsDir: "testdata/fragments/symlinks",
			wantErr:      "escapes",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidatePR(tt.input, tt.fragmentsDir)
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
