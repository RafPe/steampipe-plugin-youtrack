package release

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

// Exit codes returned by Run.
const (
	exitSuccess          = 0
	exitValidationFailed = 1
	exitUsageOrIOError   = 2
)

// maxInputBytes bounds the size of an --input document (file or stdin).
// Larger input is rejected outright, before any JSON parsing.
const maxInputBytes = 1 << 20 // 1 MiB

const defaultFragmentsDir = ".changes/unreleased"

// Run implements the releasectl CLI: it parses args, reads the --input
// document from stdin or a file, executes the requested subcommand, and
// writes results to stdout/stderr. The returned int is the process exit
// code: 0 on success, 1 on a validation failure, 2 on a usage or I/O
// error. cmd/releasectl/main.go is expected to call
// os.Exit(release.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) and
// contain no other logic.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printErr(stderr, "usage: releasectl validate-pr|next-version --input <file|-> --trusted-repo <owner/name> [flags], or validate-fragments --dir <dir>")
		return exitUsageOrIOError
	}

	switch args[0] {
	case "validate-pr":
		return runValidatePR(args[1:], stdin, stdout, stderr)
	case "next-version":
		return runNextVersion(args[1:], stdin, stdout, stderr)
	case "validate-fragments":
		return runValidateFragments(args[1:], stdout, stderr)
	default:
		printErr(stderr, fmt.Sprintf("unknown subcommand %q", args[0]))
		return exitUsageOrIOError
	}
}

func runValidatePR(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate-pr", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	input := fs.String("input", "", "input JSON file path, or - for stdin")
	fragmentsDir := fs.String("fragments-dir", defaultFragmentsDir, "changelog fragments directory")
	trustedRepo := fs.String("trusted-repo", "", "trusted base repository (owner/name) generated release PRs must originate from")
	if err := fs.Parse(args); err != nil {
		printErr(stderr, fmt.Sprintf("invalid flags: %v", err))
		return exitUsageOrIOError
	}
	if *input == "" {
		printErr(stderr, "--input is required")
		return exitUsageOrIOError
	}
	if *trustedRepo == "" {
		printErr(stderr, "--trusted-repo is required")
		return exitUsageOrIOError
	}

	data, err := readInput(*input, stdin)
	if err != nil {
		printErr(stderr, err.Error())
		return exitUsageOrIOError
	}

	payload, err := decodeValidatePRInput(data)
	if err != nil {
		printErr(stderr, err.Error())
		return exitUsageOrIOError
	}

	message, err := ValidatePR(payload, *fragmentsDir, *trustedRepo)
	if err != nil {
		printErr(stderr, err.Error())
		return exitValidationFailed
	}
	if _, err := fmt.Fprintln(stdout, message); err != nil {
		printErr(stderr, fmt.Sprintf("write output: %v", err))
		return exitUsageOrIOError
	}
	return exitSuccess
}

func runNextVersion(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("next-version", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	input := fs.String("input", "", "input JSON file path, or - for stdin")
	trustedRepo := fs.String("trusted-repo", "", "trusted base repository (owner/name) generated release PRs must originate from")
	if err := fs.Parse(args); err != nil {
		printErr(stderr, fmt.Sprintf("invalid flags: %v", err))
		return exitUsageOrIOError
	}
	if *input == "" {
		printErr(stderr, "--input is required")
		return exitUsageOrIOError
	}
	if *trustedRepo == "" {
		printErr(stderr, "--trusted-repo is required")
		return exitUsageOrIOError
	}

	data, err := readInput(*input, stdin)
	if err != nil {
		printErr(stderr, err.Error())
		return exitUsageOrIOError
	}

	payload, err := decodeNextVersionInput(data)
	if err != nil {
		printErr(stderr, err.Error())
		return exitUsageOrIOError
	}

	result, err := NextVersion(payload, *trustedRepo)
	if err != nil {
		printErr(stderr, err.Error())
		return exitValidationFailed
	}

	// next-version's stdout contract is JSON-only (no other output), so
	// callers can pipe it straight into `jq` or similar.
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		printErr(stderr, fmt.Sprintf("write output: %v", err))
		return exitUsageOrIOError
	}
	return exitSuccess
}

func runValidateFragments(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate-fragments", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("dir", defaultFragmentsDir, "changelog fragments directory to validate")
	if err := fs.Parse(args); err != nil {
		printErr(stderr, fmt.Sprintf("invalid flags: %v", err))
		return exitUsageOrIOError
	}

	files, err := listFragmentFiles(*dir)
	if err != nil {
		printErr(stderr, err.Error())
		return exitUsageOrIOError
	}

	for _, f := range files {
		if err := validateFragmentFile(*dir, f); err != nil {
			printErr(stderr, err.Error())
			return exitValidationFailed
		}
	}

	if _, err := fmt.Fprintf(stdout, "ok: %d fragment(s) validated in %s\n", len(files), *dir); err != nil {
		printErr(stderr, fmt.Sprintf("write output: %v", err))
		return exitUsageOrIOError
	}
	return exitSuccess
}

// readInput reads an --input document from path ("-" for stdin) or a file,
// rejecting documents larger than maxInputBytes before any parsing.
func readInput(path string, stdin io.Reader) ([]byte, error) {
	var r io.Reader
	if path == "-" {
		r = stdin
	} else {
		f, err := os.Open(path) // #nosec G304 -- path is an operator-supplied CLI flag, not untrusted network input
		if err != nil {
			return nil, fmt.Errorf("open input: %w", err)
		}
		defer func() { _ = f.Close() }()
		r = f
	}

	data, err := io.ReadAll(io.LimitReader(r, maxInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	if len(data) > maxInputBytes {
		return nil, fmt.Errorf("input exceeds %d byte limit", maxInputBytes)
	}
	return data, nil
}

// decodeStrictJSON decodes exactly one JSON value from data into v,
// rejecting any field in the document that doesn't correspond to a field
// in v's type and rejecting any non-whitespace content following that one
// value (e.g. a second concatenated JSON document).
func decodeStrictJSON(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid input json: %w", err)
	}
	if dec.More() {
		return fmt.Errorf("invalid input json: trailing content after json value")
	}
	return nil
}

// wireValidatePRInput mirrors ValidatePRInput but with pointer fields, so
// decodeValidatePRInput can distinguish a wholly absent JSON key (a
// malformed caller -- rejected) from a key explicitly present with a
// zero-ish value (e.g. an empty "labels" array, which is a normal,
// validatable PR state: zero release labels is ValidatePR's job to reject,
// not decodeValidatePRInput's).
type wireValidatePRInput struct {
	Labels       *[]string `json:"labels"`
	HeadBranch   *string   `json:"head_branch"`
	HeadRepo     *string   `json:"head_repo"`
	ChangedFiles *[]string `json:"changed_files"`
}

func decodeValidatePRInput(data []byte) (ValidatePRInput, error) {
	var w wireValidatePRInput
	if err := decodeStrictJSON(data, &w); err != nil {
		return ValidatePRInput{}, err
	}
	switch {
	case w.Labels == nil:
		return ValidatePRInput{}, fmt.Errorf("missing required field %q", "labels")
	case w.HeadBranch == nil:
		return ValidatePRInput{}, fmt.Errorf("missing required field %q", "head_branch")
	case w.HeadRepo == nil:
		return ValidatePRInput{}, fmt.Errorf("missing required field %q", "head_repo")
	case w.ChangedFiles == nil:
		return ValidatePRInput{}, fmt.Errorf("missing required field %q", "changed_files")
	}
	return ValidatePRInput{
		Labels:       *w.Labels,
		HeadBranch:   *w.HeadBranch,
		HeadRepo:     *w.HeadRepo,
		ChangedFiles: *w.ChangedFiles,
	}, nil
}

// wirePRInfo is wireValidatePRInput's counterpart for a single entry in
// NextVersionInput.PRs. Number is intentionally a plain int, not a
// presence-checked pointer: it is only used to name a PR in an error
// message, so an absent key defaulting to 0 is harmless.
type wirePRInfo struct {
	Number     int       `json:"number"`
	Labels     *[]string `json:"labels"`
	HeadBranch *string   `json:"head_branch"`
	HeadRepo   *string   `json:"head_repo"`
}

// wireNextVersionInput is wireValidatePRInput's counterpart for
// NextVersionInput. PreviousTag stays a plain string (not presence-checked):
// an absent key and an explicitly empty string are both the documented
// bootstrap trigger (see NextVersion), so there is nothing to distinguish.
type wireNextVersionInput struct {
	PreviousTag string        `json:"previous_tag"`
	PRs         *[]wirePRInfo `json:"prs"`
}

func decodeNextVersionInput(data []byte) (NextVersionInput, error) {
	var w wireNextVersionInput
	if err := decodeStrictJSON(data, &w); err != nil {
		return NextVersionInput{}, err
	}
	if w.PRs == nil {
		return NextVersionInput{}, fmt.Errorf("missing required field %q", "prs")
	}

	prs := make([]PRInfo, len(*w.PRs))
	for i, wp := range *w.PRs {
		switch {
		case wp.Labels == nil:
			return NextVersionInput{}, fmt.Errorf("prs[%d]: missing required field %q", i, "labels")
		case wp.HeadBranch == nil:
			return NextVersionInput{}, fmt.Errorf("prs[%d]: missing required field %q", i, "head_branch")
		case wp.HeadRepo == nil:
			return NextVersionInput{}, fmt.Errorf("prs[%d]: missing required field %q", i, "head_repo")
		}
		prs[i] = PRInfo{
			Number:     wp.Number,
			Labels:     *wp.Labels,
			HeadBranch: *wp.HeadBranch,
			HeadRepo:   *wp.HeadRepo,
		}
	}

	return NextVersionInput{PreviousTag: w.PreviousTag, PRs: prs}, nil
}

// printErr writes a lowercase, human-readable error line to stderr. Errors
// writing the error message itself are deliberately not surfaced: the exit
// code the caller already chose reflects the real failure, and there is no
// more meaningful action to take if stderr itself is unwritable.
func printErr(stderr io.Writer, msg string) {
	_, _ = fmt.Fprintln(stderr, msg)
}
