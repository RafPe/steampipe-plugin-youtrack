package release

import (
	"strings"
	"testing"
)

func TestFragmentCandidates(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		dir     string
		files   []string
		want    []string
		wantErr string
	}{
		"selects yaml files under the dir": {
			dir:   "testdata/fragments/valid",
			files: []string{"testdata/fragments/valid/Added-1.yaml", "youtrack/plugin.go"},
			want:  []string{"testdata/fragments/valid/Added-1.yaml"},
		},
		"ignores non-yaml files under the dir": {
			dir:   "testdata/fragments/bad",
			files: []string{"testdata/fragments/bad/isdir.yaml/.gitkeep"},
			want:  nil,
		},
		"ignores files outside the dir": {
			dir:   "testdata/fragments/valid",
			files: []string{"testdata/fragments/outside/secret.yaml"},
			want:  nil,
		},
		"no candidates at all": {
			dir:   "testdata/fragments/valid",
			files: []string{"README.md", "go.mod"},
			want:  nil,
		},
		"redundant dotdot that stays inside is not an escape": {
			dir:   "testdata/fragments/valid",
			files: []string{"testdata/fragments/valid/sub/../Added-1.yaml"},
			want:  []string{"testdata/fragments/valid/Added-1.yaml"},
		},
		"dotdot traversal escaping the dir errors": {
			dir:     "testdata/fragments/valid",
			files:   []string{"testdata/fragments/valid/../../../etc/passwd"},
			wantErr: "escapes",
		},
		"absolute changed_files entry errors": {
			dir:     "testdata/fragments/valid",
			files:   []string{"/etc/passwd"},
			wantErr: "repo-relative",
		},
		"absolute entry errors even when unrelated to fragments dir": {
			dir:     "testdata/fragments/valid",
			files:   []string{"testdata/fragments/valid/Added-1.yaml", "/tmp/x.yaml"},
			wantErr: "repo-relative",
		},
		"empty changed_files entry errors": {
			dir:     "testdata/fragments/valid",
			files:   []string{""},
			wantErr: "must not be empty",
		},
		"empty fragments dir errors": {
			dir:     "",
			files:   []string{"testdata/fragments/valid/Added-1.yaml"},
			wantErr: "must not be empty",
		},
		"dot fragments dir errors": {
			dir:     ".",
			files:   []string{"Added-1.yaml"},
			wantErr: "must not be empty",
		},
		"no changed files": {
			dir:   "testdata/fragments/valid",
			files: nil,
			want:  nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := fragmentCandidates(tt.dir, tt.files)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("fragmentCandidates() error = %v, want nil", err)
				}
				if !equalStringSlices(got, tt.want) {
					t.Errorf("fragmentCandidates() = %v, want %v", got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("fragmentCandidates() = %v, want error containing %q", got, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("fragmentCandidates() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestValidateFragmentFile(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		dir     string
		path    string
		wantErr string
	}{
		"valid Added":        {dir: "testdata/fragments/valid", path: "testdata/fragments/valid/Added-1.yaml"},
		"valid Changed":      {dir: "testdata/fragments/valid", path: "testdata/fragments/valid/Changed-1.yaml"},
		"valid Deprecated":   {dir: "testdata/fragments/valid", path: "testdata/fragments/valid/Deprecated-1.yaml"},
		"valid Removed":      {dir: "testdata/fragments/valid", path: "testdata/fragments/valid/Removed-1.yaml"},
		"valid Fixed":        {dir: "testdata/fragments/valid", path: "testdata/fragments/valid/Fixed-1.yaml"},
		"valid Security":     {dir: "testdata/fragments/valid", path: "testdata/fragments/valid/Security-1.yaml"},
		"valid Dependencies": {dir: "testdata/fragments/valid", path: "testdata/fragments/valid/Dependencies-1.yaml"},
		"malformed yaml": {
			dir: "testdata/fragments/bad", path: "testdata/fragments/bad/malformed.yaml",
			wantErr: "invalid yaml",
		},
		"unknown kind": {
			dir: "testdata/fragments/bad", path: "testdata/fragments/bad/unknown-kind.yaml",
			wantErr: "unknown kind",
		},
		"empty body": {
			dir: "testdata/fragments/bad", path: "testdata/fragments/bad/empty-body.yaml",
			wantErr: "empty body",
		},
		"credential shape ghp": {
			dir: "testdata/fragments/bad", path: "testdata/fragments/bad/credential-ghp.yaml",
			wantErr: "credential",
		},
		"credential shape github_pat": {
			dir: "testdata/fragments/bad", path: "testdata/fragments/bad/credential-pat.yaml",
			wantErr: "credential",
		},
		"credential shape perm": {
			dir: "testdata/fragments/bad", path: "testdata/fragments/bad/credential-perm.yaml",
			wantErr: "credential",
		},
		"fragment is a directory": {
			dir: "testdata/fragments/bad", path: "testdata/fragments/bad/isdir.yaml",
			wantErr: "fragment",
		},
		"missing fragment": {
			dir: "testdata/fragments/valid", path: "testdata/fragments/valid/DoesNotExist.yaml",
			wantErr: "fragment",
		},
		"symlink escapes the fragments dir": {
			dir: "testdata/fragments/symlinks", path: "testdata/fragments/symlinks/escape.yaml",
			wantErr: "escapes",
		},
		"missing fragments dir": {
			dir: "testdata/fragments/does-not-exist", path: "testdata/fragments/does-not-exist/x.yaml",
			wantErr: "fragments directory",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateFragmentFile(tt.dir, tt.path)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("validateFragmentFile() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateFragmentFile() = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("validateFragmentFile() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

// TestValidateFragmentFileNeverEchoesCredentials guards the "WITHOUT echoing
// the matched content" requirement directly: the fake tokens in the bad
// fragment fixtures must never appear verbatim in an error message.
func TestValidateFragmentFileNeverEchoesCredentials(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path  string
		token string
	}{
		{"testdata/fragments/bad/credential-ghp.yaml", "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{"testdata/fragments/bad/credential-pat.yaml", "github_pat_11AAAAAAA0123456789ABCDEFabcdefGHIJKLMNOP"},
		{"testdata/fragments/bad/credential-perm.yaml", "perm:AbCdEfGhIjKlMnOp.QrStUvWx.YzAbCdEf"},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			err := validateFragmentFile("testdata/fragments/bad", tc.path)
			if err == nil {
				t.Fatalf("validateFragmentFile(%q) = nil, want an error", tc.path)
			}
			if strings.Contains(err.Error(), tc.token) {
				t.Errorf("validateFragmentFile(%q) error echoed the credential: %v", tc.path, err)
			}
		})
	}
}

// TestIsWithinDirSameDir exercises the defensive branch where child equals
// parent directly: not reachable through fragmentCandidates (which only
// yields ".yaml" file paths, never a bare directory), so it is unit tested
// directly.
func TestIsWithinDirSameDir(t *testing.T) {
	t.Parallel()

	if isWithinDir("testdata/fragments/valid", "testdata/fragments/valid") {
		t.Error("isWithinDir(dir, dir) = true, want false")
	}
}

// TestIsWithinDirRelError exercises filepath.Rel's error branch (mismatched
// absolute/relative paths). validateFragmentFile always resolves both sides
// through filepath.EvalSymlinks with matching absoluteness, so this
// defensive branch is unreachable through the CLI and is unit tested
// directly.
func TestIsWithinDirRelError(t *testing.T) {
	t.Parallel()

	if isWithinDir("/absolute/child", "relative/parent") {
		t.Error("isWithinDir(absolute, relative) = true, want false")
	}
}
