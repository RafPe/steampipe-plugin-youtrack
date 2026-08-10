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
		"nonexistent input file": {
			args:       []string{"validate-pr", "--input", "testdata/does-not-exist.json"},
			wantExit:   exitUsageOrIOError,
			wantStderr: "open input",
		},
		"malformed json": {
			args:       []string{"validate-pr", "--input", "-"},
			stdin:      "{not json",
			wantExit:   exitUsageOrIOError,
			wantStderr: "invalid input json",
		},
		"validation failure exits 1": {
			args:       []string{"validate-pr", "--input", "-", "--fragments-dir", "testdata/fragments/valid"},
			stdin:      `{"labels":["bug"],"head_branch":"feat/x"}`,
			wantExit:   exitValidationFailed,
			wantStderr: "no release label found",
		},
		"success from stdin exits 0": {
			args: []string{"validate-pr", "--input", "-", "--fragments-dir", "testdata/fragments/valid"},
			stdin: `{"labels":["release/patch"],"head_branch":"feat/x",` +
				`"changed_files":["testdata/fragments/valid/Added-1.yaml"]}`,
			wantExit:   exitSuccess,
			wantStdout: "ok: release/patch with 1 changelog fragment(s)\n",
		},
		"success from file exits 0": {
			args:       []string{"validate-pr", "--input", "testdata/run/validate-pr-generated.json"},
			wantExit:   exitSuccess,
			wantStdout: "ok: generated release pr, exempt from label and fragment checks\n",
		},
		"default fragments-dir is .changes/unreleased": {
			args:       []string{"validate-pr", "--input", "-"},
			stdin:      `{"head_branch":"release/next"}`,
			wantExit:   exitSuccess,
			wantStdout: "ok: generated release pr, exempt from label and fragment checks\n",
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
		"nonexistent input file": {
			args:       []string{"next-version", "--input", "testdata/does-not-exist.json"},
			wantExit:   exitUsageOrIOError,
			wantStderr: "open input",
		},
		"malformed json": {
			args:       []string{"next-version", "--input", "-"},
			stdin:      "not json",
			wantExit:   exitUsageOrIOError,
			wantStderr: "invalid input json",
		},
		"validation failure exits 1": {
			args:       []string{"next-version", "--input", "-"},
			stdin:      `{"previous_tag":"v1.0.0","prs":[{"number":9,"labels":["bug"],"head_branch":"feat/x"}]}`,
			wantExit:   exitValidationFailed,
			wantStderr: "pr #9",
		},
		"release success exits 0 with json on stdout": {
			args: []string{"next-version", "--input", "-"},
			stdin: `{"previous_tag":"v0.1.0","prs":[` +
				`{"number":12,"labels":["release/minor"],"head_branch":"feat/x"}]}`,
			wantExit:   exitSuccess,
			wantStdout: &NextVersionResult{Release: true, Version: "v0.2.0", Previous: "v0.1.0", Bump: "minor"},
		},
		"no-release result exits 0 with json on stdout": {
			args:       []string{"next-version", "--input", "-"},
			stdin:      `{"previous_tag":"v0.1.0","prs":[]}`,
			wantExit:   exitSuccess,
			wantStdout: &NextVersionResult{Release: false, Reason: "no releasable prs"},
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

func TestRunOversizedInput(t *testing.T) {
	t.Parallel()

	oversized := strings.Repeat("a", maxInputBytes+1)
	var stdout, stderr bytes.Buffer
	got := Run([]string{"validate-pr", "--input", "-"}, strings.NewReader(oversized), &stdout, &stderr)
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
	// valid, exempt validate-pr document with whitespace to hit the limit
	// exactly.
	payload := `{"head_branch":"release/next"}`
	padded := payload + strings.Repeat(" ", maxInputBytes-len(payload))
	if len(padded) != maxInputBytes {
		t.Fatalf("test setup: padded length = %d, want %d", len(padded), maxInputBytes)
	}

	var stdout, stderr bytes.Buffer
	got := Run([]string{"validate-pr", "--input", "-"}, strings.NewReader(padded), &stdout, &stderr)
	if got != exitSuccess {
		t.Errorf("Run() exit = %d, want %d (stderr=%q)", got, exitSuccess, stderr.String())
	}
}

func TestRunValidatePRFromFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "input.json")
	if err := os.WriteFile(path, []byte(`{"head_branch":"release/next"}`), 0o600); err != nil {
		t.Fatalf("write test input: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := Run([]string{"validate-pr", "--input", path}, strings.NewReader(""), &stdout, &stderr)
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
	got := Run([]string{"validate-pr", "--input", "-"}, failingReader{}, &stdout, &stderr)
	if got != exitUsageOrIOError {
		t.Errorf("Run() exit = %d, want %d", got, exitUsageOrIOError)
	}
	if !strings.Contains(stderr.String(), "read input") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "read input")
	}
}

func TestRunValidatePRStdoutWriteFailure(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	got := Run(
		[]string{"validate-pr", "--input", "-"},
		strings.NewReader(`{"head_branch":"release/next"}`),
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
		[]string{"next-version", "--input", "-"},
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
