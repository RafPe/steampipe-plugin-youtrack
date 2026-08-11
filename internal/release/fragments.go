package release

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Changie fragment kinds configured in .changie.yaml, in their declared
// order. A fragment's "kind" field must be an exact, case-sensitive match
// for one of these.
const (
	kindAdded        = "Added"
	kindChanged      = "Changed"
	kindDeprecated   = "Deprecated"
	kindRemoved      = "Removed"
	kindFixed        = "Fixed"
	kindSecurity     = "Security"
	kindDependencies = "Dependencies"
)

// orderedFragmentKinds lists the valid kinds in .changie.yaml's declared
// order, for building a helpful "want one of ..." message. validFragmentKinds
// is derived from it so the two can never drift apart.
var orderedFragmentKinds = []string{
	kindAdded, kindChanged, kindDeprecated, kindRemoved, kindFixed, kindSecurity, kindDependencies,
}

var validFragmentKinds = func() map[string]bool {
	m := make(map[string]bool, len(orderedFragmentKinds))
	for _, k := range orderedFragmentKinds {
		m[k] = true
	}
	return m
}()

// fragment mirrors the subset of a Changie fragment's YAML shape (see
// task-1-report.md) that validation needs: kind and body. The time field is
// ignored (used by Changie only for ordering, never for validation).
type fragment struct {
	Kind string `yaml:"kind"`
	Body string `yaml:"body"`
}

// fragmentCandidates scans changedFiles for entries that reference a
// changelog fragment under fragmentsDir, and rejects path-safety violations
// (absolute paths anywhere in changedFiles, or ".." traversal that escapes
// fragmentsDir). An entry counts as a candidate if it resolves inside
// fragmentsDir once cleaned (regardless of how it's written) or if its raw,
// uncleaned form is textually aimed at fragmentsDir (so a disguised ".."
// escape is reported as an error rather than silently treated as an
// unrelated file). Returned paths are cleaned, repo-relative, and directly
// usable to open the file. Only ".yaml" files are treated as fragments;
// non-YAML entries under fragmentsDir (e.g. ".gitkeep") are ignored.
func fragmentCandidates(fragmentsDir string, changedFiles []string) ([]string, error) {
	cleanDir := filepath.Clean(strings.TrimSpace(fragmentsDir))
	if cleanDir == "" || cleanDir == "." {
		return nil, fmt.Errorf("fragments directory must not be empty")
	}
	prefix := cleanDir + string(filepath.Separator)

	var candidates []string
	for _, f := range changedFiles {
		if f == "" {
			return nil, fmt.Errorf("changed_files entry must not be empty")
		}
		if filepath.IsAbs(f) {
			return nil, fmt.Errorf("changed_files entries must be repo-relative paths, got %q", f)
		}
		// Candidacy is decided two ways, deliberately not just one: the
		// cleaned form tells us where the path actually resolves, but a
		// raw path that merely *looks* aimed at the fragments dir (before
		// cleaning) must still be routed into the escape check below
		// rather than silently skipped as "unrelated" -- otherwise a
		// disguised ".." traversal would just be ignored instead of
		// rejected.
		cleaned := filepath.Clean(f)
		cleanContained := cleaned == cleanDir || strings.HasPrefix(cleaned, prefix)
		rawLooksTargeted := strings.HasPrefix(f, prefix)
		if !cleanContained && !rawLooksTargeted {
			continue // not under the fragments dir; irrelevant to fragment validation
		}
		if !cleanContained {
			return nil, fmt.Errorf("fragment path %q escapes the fragments directory", f)
		}
		if filepath.Ext(cleaned) != ".yaml" {
			continue // e.g. the fragments dir's .gitkeep
		}
		candidates = append(candidates, cleaned)
	}
	return candidates, nil
}

// listFragmentFiles returns the cleaned, repo-relative paths of every
// ".yaml" file directly inside dir (non-recursive; non-YAML entries such as
// ".gitkeep" are skipped, matching fragmentCandidates), sorted for
// deterministic output. It performs no content validation -- pair with
// validateFragmentFile per returned path, as releasectl validate-fragments
// does.
func listFragmentFiles(dir string) ([]string, error) {
	cleanDir := filepath.Clean(strings.TrimSpace(dir))
	if cleanDir == "" || cleanDir == "." {
		return nil, fmt.Errorf("fragments directory must not be empty")
	}

	entries, err := os.ReadDir(cleanDir)
	if err != nil {
		return nil, fmt.Errorf("fragments directory %q: %w", dir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		files = append(files, filepath.Join(cleanDir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// validateFragmentFile reads and validates a single fragment file at
// relPath (as returned by fragmentCandidates): it must resolve (following
// symlinks) to a location still inside fragmentsDir, exist on disk, parse
// as YAML, use a known kind, have a non-empty body, and its body must not
// match an obvious credential shape.
func validateFragmentFile(fragmentsDir, relPath string) error {
	realDir, err := filepath.EvalSymlinks(fragmentsDir)
	if err != nil {
		return fmt.Errorf("fragments directory %q: %w", fragmentsDir, err)
	}
	realFile, err := filepath.EvalSymlinks(relPath)
	if err != nil {
		return fmt.Errorf("fragment %q: %w", relPath, err)
	}
	if !isWithinDir(realFile, realDir) {
		return fmt.Errorf("fragment %q escapes the fragments directory", relPath)
	}

	data, err := os.ReadFile(realFile) // #nosec G304 -- realFile is resolved via EvalSymlinks and containment-checked above
	if err != nil {
		return fmt.Errorf("fragment %q: %w", relPath, err)
	}

	var frag fragment
	if err := yaml.Unmarshal(data, &frag); err != nil {
		return fmt.Errorf("fragment %q: invalid yaml", relPath)
	}
	if !validFragmentKinds[frag.Kind] {
		// frag.Kind is deliberately not included in this message: it's
		// attacker-controlled fragment content, and echoing it back would
		// let a credential-shaped kind value leak into stderr/CI logs the
		// same way a credential-shaped body could (see containsCredentialShape
		// below, which only scans Body). Naming the allowed kinds is more
		// actionable for a contributor anyway.
		return fmt.Errorf("fragment %q: unknown kind, want one of %s", relPath, strings.Join(orderedFragmentKinds, ", "))
	}
	if strings.TrimSpace(frag.Body) == "" {
		return fmt.Errorf("fragment %q: empty body", relPath)
	}
	if containsCredentialShape(frag.Body) {
		return fmt.Errorf("fragment %q: body appears to contain a credential and was rejected", relPath)
	}
	return nil
}

// isWithinDir reports whether child is a file strictly inside parent (both
// already resolved to real, symlink-free paths).
func isWithinDir(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return false // child is the directory itself, not a file inside it
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// credentialPatterns match obvious credential shapes that must never appear
// in a changelog fragment body: classic GitHub personal access tokens,
// GitHub fine-grained PAT prefixes, and YouTrack permanent tokens.
var credentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]+`),
	regexp.MustCompile(`perm:\S+`),
}

func containsCredentialShape(body string) bool {
	for _, p := range credentialPatterns {
		if p.MatchString(body) {
			return true
		}
	}
	return false
}
