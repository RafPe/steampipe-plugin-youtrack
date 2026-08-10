package release

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunNoArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	got := Run(nil, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsageOrIOError {
		t.Errorf("Run(nil) = %d, want %d", got, exitUsageOrIOError)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "usage:")
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	got := Run([]string{"bogus"}, strings.NewReader(""), &stdout, &stderr)
	if got != exitUsageOrIOError {
		t.Errorf("Run([bogus]) = %d, want %d", got, exitUsageOrIOError)
	}
	if !strings.Contains(stderr.String(), `unknown subcommand "bogus"`) {
		t.Errorf("stderr = %q, want it to contain the subcommand name", stderr.String())
	}
}

func TestRunValidatePR(t *testing.T) {
	t.Parallel()

	trustedRepoFlag := []string{"--trusted-repo", trustedRepoForTests}

	tests := map[string]struct {
		args       []string
		stdin      string
		wantExit   int
		wantStdout string
		wantStderr string
	}{
		"missing --input": {
			args:       []string{"validate-pr"},
			wantExit:   exitUsageOrIOError,
			wantStderr: "--input is required",
		},
		"invalid flag": {
			args:       []string{"validate-pr", "--bogus"},
			wantExit:   exitUsageOrIOError,
			wantStderr: "invalid flags",
		},
		"missing --trusted-repo": {
			args:       []string{"validate-pr", "--input", "-"},
			stdin:      `{"labels":[],"head_branch":"feat/x","head_repo":"x/y","changed_files":[]}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: "--trusted-repo is required",
		},
		"nonexistent input file": {
			args:       append([]string{"validate-pr", "--input", "testdata/does-not-exist.json"}, trustedRepoFlag...),
			wantExit:   exitUsageOrIOError,
			wantStderr: "open input",
		},
		"malformed json": {
			args:       append([]string{"validate-pr", "--input", "-"}, trustedRepoFlag...),
			stdin:      "{not json",
			wantExit:   exitUsageOrIOError,
			wantStderr: "invalid input json",
		},
		"unknown field rejected": {
			args: append([]string{"validate-pr", "--input", "-"}, trustedRepoFlag...),
			stdin: `{"labels":[],"head_branch":"x","head_repo":"y","changed_files":[],` +
				`"bogus_field":true}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: "unknown field",
		},
		"trailing content after json value rejected": {
			args: append([]string{"validate-pr", "--input", "-"}, trustedRepoFlag...),
			stdin: `{"labels":[],"head_branch":"x","head_repo":"y","changed_files":[]}` +
				`{"extra":1}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: "trailing content",
		},
		"missing labels field": {
			args:       append([]string{"validate-pr", "--input", "-"}, trustedRepoFlag...),
			stdin:      `{"head_branch":"x","head_repo":"y","changed_files":[]}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: `missing required field "labels"`,
		},
		"missing head_branch field": {
			args:       append([]string{"validate-pr", "--input", "-"}, trustedRepoFlag...),
			stdin:      `{"labels":[],"head_repo":"y","changed_files":[]}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: `missing required field "head_branch"`,
		},
		"missing head_repo field": {
			args:       append([]string{"validate-pr", "--input", "-"}, trustedRepoFlag...),
			stdin:      `{"labels":[],"head_branch":"x","changed_files":[]}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: `missing required field "head_repo"`,
		},
		"missing changed_files field": {
			args:       append([]string{"validate-pr", "--input", "-"}, trustedRepoFlag...),
			stdin:      `{"labels":[],"head_branch":"x","head_repo":"y"}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: `missing required field "changed_files"`,
		},
		"validation failure exits 1": {
			args: append([]string{"validate-pr", "--input", "-", "--fragments-dir", "testdata/fragments/valid"}, trustedRepoFlag...),
			stdin: `{"labels":["bug"],"head_branch":"feat/x","head_repo":"` + trustedRepoForTests +
				`","changed_files":[]}`,
			wantExit:   exitValidationFailed,
			wantStderr: "no release label found",
		},
		"success from stdin exits 0": {
			args: append([]string{"validate-pr", "--input", "-", "--fragments-dir", "testdata/fragments/valid"}, trustedRepoFlag...),
			stdin: `{"labels":["release/patch"],"head_branch":"feat/x","head_repo":"` + trustedRepoForTests + `",` +
				`"changed_files":["testdata/fragments/valid/Added-1.yaml"]}`,
			wantExit:   exitSuccess,
			wantStdout: "ok: release/patch with 1 changelog fragment(s)\n",
		},
		"success from file exits 0": {
			args:       append([]string{"validate-pr", "--input", "testdata/run/validate-pr-generated.json"}, trustedRepoFlag...),
			wantExit:   exitSuccess,
			wantStdout: "ok: generated release pr, exempt from label and fragment checks\n",
		},
		"default fragments-dir is .changes/unreleased": {
			args: append([]string{"validate-pr", "--input", "-"}, trustedRepoFlag...),
			stdin: `{"labels":["autorelease: pending"],"head_branch":"release/next","head_repo":"` +
				trustedRepoForTests + `","changed_files":[]}`,
			wantExit:   exitSuccess,
			wantStdout: "ok: generated release pr, exempt from label and fragment checks\n",
		},
		"fork cannot spoof exemption via the cli": {
			args: append([]string{"validate-pr", "--input", "-"}, trustedRepoFlag...),
			stdin: `{"labels":["autorelease: pending"],"head_branch":"release/next","head_repo":"` +
				forkRepoForTests + `","changed_files":[]}`,
			wantExit:   exitValidationFailed,
			wantStderr: "no release label found",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			got := Run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
			if got != tt.wantExit {
				t.Errorf("Run() exit = %d, want %d (stdout=%q stderr=%q)", got, tt.wantExit, stdout.String(), stderr.String())
			}
			if tt.wantStdout != "" && stdout.String() != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunNextVersion(t *testing.T) {
	t.Parallel()

	trustedRepoFlag := []string{"--trusted-repo", trustedRepoForTests}

	tests := map[string]struct {
		args       []string
		stdin      string
		wantExit   int
		wantStdout *NextVersionResult
		wantStderr string
	}{
		"missing --input": {
			args:       []string{"next-version"},
			wantExit:   exitUsageOrIOError,
			wantStderr: "--input is required",
		},
		"invalid flag": {
			args:       []string{"next-version", "--bogus"},
			wantExit:   exitUsageOrIOError,
			wantStderr: "invalid flags",
		},
		"missing --trusted-repo": {
			args:       []string{"next-version", "--input", "-"},
			stdin:      `{"prs":[]}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: "--trusted-repo is required",
		},
		"nonexistent input file": {
			args:       append([]string{"next-version", "--input", "testdata/does-not-exist.json"}, trustedRepoFlag...),
			wantExit:   exitUsageOrIOError,
			wantStderr: "open input",
		},
		"malformed json": {
			args:       append([]string{"next-version", "--input", "-"}, trustedRepoFlag...),
			stdin:      "not json",
			wantExit:   exitUsageOrIOError,
			wantStderr: "invalid input json",
		},
		"unknown field rejected": {
			args:       append([]string{"next-version", "--input", "-"}, trustedRepoFlag...),
			stdin:      `{"prs":[],"bogus_field":true}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: "unknown field",
		},
		"trailing content after json value rejected": {
			args:       append([]string{"next-version", "--input", "-"}, trustedRepoFlag...),
			stdin:      `{"prs":[]}{"extra":1}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: "trailing content",
		},
		"missing prs field": {
			args:       append([]string{"next-version", "--input", "-"}, trustedRepoFlag...),
			stdin:      `{"previous_tag":"v1.0.0"}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: `missing required field "prs"`,
		},
		"missing labels in a pr entry": {
			args:       append([]string{"next-version", "--input", "-"}, trustedRepoFlag...),
			stdin:      `{"prs":[{"number":1,"head_branch":"x","head_repo":"y"}]}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: `prs[0]: missing required field "labels"`,
		},
		"missing head_branch in a pr entry": {
			args:       append([]string{"next-version", "--input", "-"}, trustedRepoFlag...),
			stdin:      `{"prs":[{"number":1,"labels":[],"head_repo":"y"}]}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: `prs[0]: missing required field "head_branch"`,
		},
		"missing head_repo in a pr entry": {
			args:       append([]string{"next-version", "--input", "-"}, trustedRepoFlag...),
			stdin:      `{"prs":[{"number":1,"labels":[],"head_branch":"x"}]}`,
			wantExit:   exitUsageOrIOError,
			wantStderr: `prs[0]: missing required field "head_repo"`,
		},
		"validation failure exits 1": {
			args: append([]string{"next-version", "--input", "-"}, trustedRepoFlag...),
			stdin: `{"previous_tag":"v1.0.0","prs":[{"number":9,"labels":["bug"],"head_branch":"feat/x",` +
				`"head_repo":"` + trustedRepoForTests + `"}]}`,
			wantExit:   exitValidationFailed,
			wantStderr: "pr #9",
		},
		"release success exits 0 with json on stdout": {
			args: append([]string{"next-version", "--input", "-"}, trustedRepoFlag...),
			stdin: `{"previous_tag":"v0.1.0","prs":[` +
				`{"number":12,"labels":["release/minor"],"head_branch":"feat/x","head_repo":"` +
				trustedRepoForTests + `"}]}`,
			wantExit:   exitSuccess,
			wantStdout: &NextVersionResult{Release: true, Version: "v0.2.0", Previous: "v0.1.0", Bump: "minor"},
		},
		"no-release result exits 0 with json on stdout": {
			args:       append([]string{"next-version", "--input", "-"}, trustedRepoFlag...),
			stdin:      `{"previous_tag":"v0.1.0","prs":[]}`,
			wantExit:   exitSuccess,
			wantStdout: &NextVersionResult{Release: false, Reason: "no releasable prs"},
		},
		"fork cannot spoof exemption via the cli": {
			// Branch and label alone match a generated release PR, but the
			// head repo doesn't -- so this PR must be validated normally,
			// not silently ignored. It carries no real release/* label, so
			// it correctly fails classifyReleaseLabel rather than slipping
			// through as an ignored generated PR.
			args: append([]string{"next-version", "--input", "-"}, trustedRepoFlag...),
			stdin: `{"previous_tag":"v1.0.0","prs":[{"number":13,` +
				`"labels":["autorelease: pending"],"head_branch":"release/next",` +
				`"head_repo":"` + forkRepoForTests + `"}]}`,
			wantExit:   exitValidationFailed,
			wantStderr: "pr #13",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			got := Run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
			if got != tt.wantExit {
				t.Errorf("Run() exit = %d, want %d (stdout=%q stderr=%q)", got, tt.wantExit, stdout.String(), stderr.String())
			}
			if tt.wantStdout != nil {
				var got NextVersionResult
				if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
					t.Fatalf("stdout is not valid json: %v (stdout=%q)", err, stdout.String())
				}
				if got != *tt.wantStdout {
					t.Errorf("stdout json = %+v, want %+v", got, *tt.wantStdout)
				}
				// The stdout contract is JSON-only: exactly one encoded
				// object, nothing else mixed in.
				if strings.Count(stdout.String(), "\n") != 1 {
					t.Errorf("stdout = %q, want exactly one line of JSON", stdout.String())
				}
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunValidateFragments(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args       []string
		wantExit   int
		wantStdout string
		wantStderr string
	}{
		"invalid flag": {
			args:       []string{"validate-fragments", "--bogus"},
			wantExit:   exitUsageOrIOError,
			wantStderr: "invalid flags",
		},
		"nonexistent dir": {
			args:       []string{"validate-fragments", "--dir", "testdata/fragments/does-not-exist"},
			wantExit:   exitUsageOrIOError,
			wantStderr: "fragments directory",
		},
		"content violation exits 1": {
			// testdata/fragments/bad's alphabetically-first fragment is
			// empty-body.yaml (the credential-shaped fixtures are generated
			// at runtime instead of committed -- see
			// TestRunValidateFragmentsCredentialShape below -- so they no
			// longer live in this directory).
			args:       []string{"validate-fragments", "--dir", "testdata/fragments/bad"},
			wantExit:   exitValidationFailed,
			wantStderr: "empty body",
		},
		"success exits 0": {
			args:       []string{"validate-fragments", "--dir", "testdata/fragments/valid"},
			wantExit:   exitSuccess,
			wantStdout: "ok: 7 fragment(s) validated in testdata/fragments/valid\n",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			got := Run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if got != tt.wantExit {
				t.Errorf("Run() exit = %d, want %d (stdout=%q stderr=%q)", got, tt.wantExit, stdout.String(), stderr.String())
			}
			if tt.wantStdout != "" && stdout.String() != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

// TestRunValidateFragmentsCredentialShape exercises the credential-shape
// rejection through the full CLI (Run -> validate-fragments ->
// listFragmentFiles -> validateFragmentFile), preserving the coverage the
// "content violation exits 1" case above used to get incidentally from the
// now-removed committed credential-*.yaml fixtures. The fixture is
// generated at runtime (see fakeCredentialShape in fragments_test.go) into
// a t.TempDir() -- validate-fragments's --dir has no relative-path
// requirement, unlike validate-pr's changed_files -- so nothing
// credential-shaped is ever committed.
func TestRunValidateFragmentsCredentialShape(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	token := fakeCredentialShape("github_pat")
	writeFragmentFile(t, dir, "Added-1.yaml", kindAdded, "Rotate token "+token+" please.")

	var stdout, stderr bytes.Buffer
	got := Run([]string{"validate-fragments", "--dir", dir}, strings.NewReader(""), &stdout, &stderr)
	if got != exitValidationFailed {
		t.Errorf("Run() exit = %d, want %d (stdout=%q stderr=%q)", got, exitValidationFailed, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "credential") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "credential")
	}
	if strings.Contains(stderr.String(), token) {
		t.Errorf("stderr = %q, must not echo the credential", stderr.String())
	}
}

func TestRunValidateFragmentsStdoutWriteFailure(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	got := Run([]string{"validate-fragments", "--dir", "testdata/fragments/valid"}, strings.NewReader(""), failingWriter{}, &stderr)
	if got != exitUsageOrIOError {
		t.Errorf("Run() exit = %d, want %d", got, exitUsageOrIOError)
	}
	if !strings.Contains(stderr.String(), "write output") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "write output")
	}
}

func TestRunOversizedInput(t *testing.T) {
	t.Parallel()

	oversized := strings.Repeat("a", maxInputBytes+1)
	var stdout, stderr bytes.Buffer
	got := Run(
		[]string{"validate-pr", "--input", "-", "--trusted-repo", trustedRepoForTests},
		strings.NewReader(oversized), &stdout, &stderr,
	)
	if got != exitUsageOrIOError {
		t.Errorf("Run() exit = %d, want %d", got, exitUsageOrIOError)
	}
	if !strings.Contains(stderr.String(), "exceeds") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "exceeds")
	}
}

func TestRunInputExactlyAtLimit(t *testing.T) {
	t.Parallel()

	// A document at exactly the byte limit must not be rejected: only
	// documents strictly larger than maxInputBytes are an error. Pad a
	// valid, exempt validate-pr document (satisfying every required field)
	// with trailing whitespace to hit the limit exactly; decodeStrictJSON's
	// dec.More() check must tolerate trailing whitespace, only rejecting
	// genuine trailing content.
	payload := `{"labels":["autorelease: pending"],"head_branch":"release/next","head_repo":"` +
		trustedRepoForTests + `","changed_files":[]}`
	padded := payload + strings.Repeat(" ", maxInputBytes-len(payload))
	if len(padded) != maxInputBytes {
		t.Fatalf("test setup: padded length = %d, want %d", len(padded), maxInputBytes)
	}

	var stdout, stderr bytes.Buffer
	got := Run(
		[]string{"validate-pr", "--input", "-", "--trusted-repo", trustedRepoForTests},
		strings.NewReader(padded), &stdout, &stderr,
	)
	if got != exitSuccess {
		t.Errorf("Run() exit = %d, want %d (stderr=%q)", got, exitSuccess, stderr.String())
	}
}

func TestRunValidatePRFromFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "input.json")
	content := `{"labels":["autorelease: pending"],"head_branch":"release/next","head_repo":"` +
		trustedRepoForTests + `","changed_files":[]}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test input: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := Run(
		[]string{"validate-pr", "--input", path, "--trusted-repo", trustedRepoForTests},
		strings.NewReader(""), &stdout, &stderr,
	)
	if got != exitSuccess {
		t.Errorf("Run() exit = %d, want %d (stderr=%q)", got, exitSuccess, stderr.String())
	}
}

// failingWriter always fails, used to exercise output-write error paths
// that cannot otherwise be triggered (the CLI's own output is always
// well-formed).
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

// failingReader always fails, used to exercise readInput's io.ReadAll error
// path (distinct from the "input exceeds the byte limit" path).
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestRunValidatePRStdinReadFailure(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	got := Run(
		[]string{"validate-pr", "--input", "-", "--trusted-repo", trustedRepoForTests},
		failingReader{}, &stdout, &stderr,
	)
	if got != exitUsageOrIOError {
		t.Errorf("Run() exit = %d, want %d", got, exitUsageOrIOError)
	}
	if !strings.Contains(stderr.String(), "read input") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "read input")
	}
}

func TestRunValidatePRStdoutWriteFailure(t *testing.T) {
	t.Parallel()

	stdin := `{"labels":["autorelease: pending"],"head_branch":"release/next","head_repo":"` +
		trustedRepoForTests + `","changed_files":[]}`
	var stderr bytes.Buffer
	got := Run(
		[]string{"validate-pr", "--input", "-", "--trusted-repo", trustedRepoForTests},
		strings.NewReader(stdin),
		failingWriter{},
		&stderr,
	)
	if got != exitUsageOrIOError {
		t.Errorf("Run() exit = %d, want %d", got, exitUsageOrIOError)
	}
	if !strings.Contains(stderr.String(), "write output") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "write output")
	}
}

func TestRunNextVersionStdoutWriteFailure(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	got := Run(
		[]string{"next-version", "--input", "-", "--trusted-repo", trustedRepoForTests},
		strings.NewReader(`{"previous_tag":"v0.1.0","prs":[]}`),
		failingWriter{},
		&stderr,
	)
	if got != exitUsageOrIOError {
		t.Errorf("Run() exit = %d, want %d", got, exitUsageOrIOError)
	}
	if !strings.Contains(stderr.String(), "write output") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "write output")
	}
}
