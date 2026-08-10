package release

import (
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
		printErr(stderr, "usage: releasectl <validate-pr|next-version> --input <file|-> [flags]")
		return exitUsageOrIOError
	}

	switch args[0] {
	case "validate-pr":
		return runValidatePR(args[1:], stdin, stdout, stderr)
	case "next-version":
		return runNextVersion(args[1:], stdin, stdout, stderr)
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
	if err := fs.Parse(args); err != nil {
		printErr(stderr, fmt.Sprintf("invalid flags: %v", err))
		return exitUsageOrIOError
	}
	if *input == "" {
		printErr(stderr, "--input is required")
		return exitUsageOrIOError
	}

	data, err := readInput(*input, stdin)
	if err != nil {
		printErr(stderr, err.Error())
		return exitUsageOrIOError
	}

	var payload ValidatePRInput
	if err := json.Unmarshal(data, &payload); err != nil {
		printErr(stderr, fmt.Sprintf("invalid input json: %v", err))
		return exitUsageOrIOError
	}

	message, err := ValidatePR(payload, *fragmentsDir)
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
	if err := fs.Parse(args); err != nil {
		printErr(stderr, fmt.Sprintf("invalid flags: %v", err))
		return exitUsageOrIOError
	}
	if *input == "" {
		printErr(stderr, "--input is required")
		return exitUsageOrIOError
	}

	data, err := readInput(*input, stdin)
	if err != nil {
		printErr(stderr, err.Error())
		return exitUsageOrIOError
	}

	var payload NextVersionInput
	if err := json.Unmarshal(data, &payload); err != nil {
		printErr(stderr, fmt.Sprintf("invalid input json: %v", err))
		return exitUsageOrIOError
	}

	result, err := NextVersion(payload)
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

// printErr writes a lowercase, human-readable error line to stderr. Errors
// writing the error message itself are deliberately not surfaced: the exit
// code the caller already chose reflects the real failure, and there is no
// more meaningful action to take if stderr itself is unwritable.
func printErr(stderr io.Writer, msg string) {
	_, _ = fmt.Fprintln(stderr, msg)
}
