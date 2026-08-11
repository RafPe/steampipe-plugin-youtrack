package release

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeCredentialShape builds a synthetic value matching one of the
// credential shapes containsCredentialShape rejects, assembled from
// disjoint parts at runtime rather than written as a realistic-looking
// literal in source. A literal like "ghp_" followed by 36 contiguous
// alphanumeric characters is exactly the shape GitHub's own push
// protection scans for, so no such literal is committed anywhere in this
// repository -- the concatenated value exists only in memory and in a
// t.TempDir() fixture that is never committed.
func fakeCredentialShape(shape string) string {
	switch shape {
	case "ghp":
		return "ghp" + "_" + strings.Repeat("x", 36)
	case "github_pat":
		return "github" + "_pat_" + strings.Repeat("x", 40)
	case "perm":
		return "perm" + ":" + strings.Repeat("x", 24)
	default:
		return ""
	}
}

// writeFragmentFile writes a fragment YAML file named name inside dir
// (created by the caller, typically via t.TempDir() so nothing is
// committed) with the given kind and body, and returns its path. Both
// fields are YAML-quoted so arbitrary content (including a
// credential-shaped kind, which contains no YAML-significant characters
// today but might) can never corrupt the fragment's structure.
func writeFragmentFile(t *testing.T, dir, name, kind, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := "kind: " + strconv.Quote(kind) + "\nbody: " + strconv.Quote(body) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fragment fixture %s: %v", path, err)
	}
	return path
}

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
		"ignores the production .gitkeep path": {
			dir:   ".changes/unreleased",
			files: []string{".changes/unreleased/.gitkeep"},
			want:  nil,
		},
		"a path not raw-prefixed but resolving inside the dir is still a candidate": {
			dir:   "testdata/fragments/valid",
			files: []string{"testdata/fragments/bad/../valid/Added-1.yaml"},
			want:  []string{"testdata/fragments/valid/Added-1.yaml"},
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

func TestListFragmentFiles(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		dir     string
		want    []string
		wantErr string
	}{
		"lists only .yaml files, sorted, skipping non-yaml and directories": {
			dir: "testdata/fragments/bad",
			want: []string{
				"testdata/fragments/bad/empty-body.yaml",
				"testdata/fragments/bad/malformed.yaml",
				"testdata/fragments/bad/unknown-kind.yaml",
			},
		},
		"empty dir": {
			dir:     "",
			wantErr: "must not be empty",
		},
		"dot dir": {
			dir:     ".",
			wantErr: "must not be empty",
		},
		"nonexistent dir": {
			dir:     "testdata/fragments/does-not-exist",
			wantErr: "fragments directory",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := listFragmentFiles(tt.dir)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("listFragmentFiles(%q) error = %v, want nil", tt.dir, err)
				}
				if !equalStringSlices(got, tt.want) {
					t.Errorf("listFragmentFiles(%q) = %v, want %v", tt.dir, got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("listFragmentFiles(%q) = %v, want error containing %q", tt.dir, got, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("listFragmentFiles(%q) error = %v, want containing %q", tt.dir, err, tt.wantErr)
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

// TestValidateFragmentFileCredentialShapes covers both the "obvious
// credential shapes are rejected" requirement and the "WITHOUT echoing the
// matched content" requirement together: for each shape, a fixture is
// generated at runtime (never committed -- see fakeCredentialShape) into a
// t.TempDir(), and the resulting error must both mention "credential" and
// never contain the fake token verbatim.
func TestValidateFragmentFileCredentialShapes(t *testing.T) {
	t.Parallel()

	for _, shape := range []string{"ghp", "github_pat", "perm"} {
		t.Run(shape, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			token := fakeCredentialShape(shape)
			path := writeFragmentFile(t, dir, "credential.yaml", kindAdded, "Rotate token "+token+" please.")

			err := validateFragmentFile(dir, path)
			if err == nil {
				t.Fatalf("validateFragmentFile() = nil, want an error for a %s-shaped credential", shape)
			}
			if !strings.Contains(err.Error(), "credential") {
				t.Errorf("validateFragmentFile() error = %v, want containing %q", err, "credential")
			}
			if strings.Contains(err.Error(), token) {
				t.Errorf("validateFragmentFile() error echoed the credential: %v", err)
			}
		})
	}
}

// TestValidateFragmentFileNeverEchoesCredentialShapedKind guards the same
// "WITHOUT echoing" requirement for the kind field: an unknown-kind error
// used to interpolate frag.Kind verbatim, which would leak a
// credential-shaped kind value into stderr/CI logs exactly like an
// unredacted body would.
func TestValidateFragmentFileNeverEchoesCredentialShapedKind(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	token := fakeCredentialShape("ghp")
	path := writeFragmentFile(t, dir, "bad-kind.yaml", token, "A normal, non-empty body.")

	err := validateFragmentFile(dir, path)
	if err == nil {
		t.Fatal("validateFragmentFile() = nil, want an error for an unknown kind")
	}
	if !strings.Contains(err.Error(), "unknown kind") {
		t.Errorf("validateFragmentFile() error = %v, want containing %q", err, "unknown kind")
	}
	if strings.Contains(err.Error(), token) {
		t.Errorf("validateFragmentFile() error echoed the kind value: %v", err)
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
