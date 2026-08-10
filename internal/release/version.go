package release

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// SemVer is a strict, parsed "vMAJOR.MINOR.PATCH" version with no
// prerelease or build metadata.
type SemVer struct {
	Major int32
	Minor int32
	Patch int32
}

// String renders v as "vMAJOR.MINOR.PATCH".
func (v SemVer) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Bump kind names accepted by SemVer.Bump.
const (
	bumpMajor = "major"
	bumpMinor = "minor"
	bumpPatch = "patch"
)

// ParseSemVer parses a strict "vMAJOR.MINOR.PATCH" version string: a
// mandatory lowercase "v" prefix, three dot-separated non-negative decimal
// integers with no leading zeros (other than the literal digit "0"), no
// prerelease or build metadata suffix, and each component fitting in an
// int32.
func ParseSemVer(s string) (SemVer, error) {
	if !strings.HasPrefix(s, "v") {
		return SemVer{}, fmt.Errorf("version %q must start with a %q prefix", s, "v")
	}
	parts := strings.Split(s[1:], ".")
	if len(parts) != 3 {
		return SemVer{}, fmt.Errorf("version %q is invalid: want vMAJOR.MINOR.PATCH", s)
	}
	components := make([]int32, 3)
	for i, part := range parts {
		n, err := parseVersionComponent(part)
		if err != nil {
			return SemVer{}, fmt.Errorf("version %q is invalid: %w", s, err)
		}
		components[i] = n
	}
	return SemVer{Major: components[0], Minor: components[1], Patch: components[2]}, nil
}

// parseVersionComponent parses a single SemVer numeric identifier: only
// ASCII digits, no leading zero unless the value is exactly "0", and must
// fit in an int32.
func parseVersionComponent(s string) (int32, error) {
	if s == "" {
		return 0, fmt.Errorf("empty version component")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("version component %q is not a non-negative integer", s)
		}
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, fmt.Errorf("version component %q has a leading zero", s)
	}
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("version component %q is out of range: %w", s, err)
	}
	return int32(n), nil
}

// Bump returns the version resulting from applying the given bump kind
// ("major", "minor", or "patch") to v, following standard SemVer semantics
// (applies uniformly regardless of whether v is pre-1.0).
func (v SemVer) Bump(kind string) (SemVer, error) {
	switch kind {
	case bumpMajor:
		return incComponent(v, "major", v.Major, func(n int32) SemVer { return SemVer{Major: n} })
	case bumpMinor:
		return incComponent(v, "minor", v.Minor, func(n int32) SemVer { return SemVer{Major: v.Major, Minor: n} })
	case bumpPatch:
		return incComponent(v, "patch", v.Patch, func(n int32) SemVer { return SemVer{Major: v.Major, Minor: v.Minor, Patch: n} })
	default:
		return SemVer{}, fmt.Errorf("invalid bump kind %q", kind)
	}
}

// incComponent increments a single version component by one, resetting the
// components that build accepts as zero via its closure, and rejects
// overflow past int32's range.
func incComponent(v SemVer, name string, current int32, build func(int32) SemVer) (SemVer, error) {
	if current == math.MaxInt32 {
		return SemVer{}, fmt.Errorf("%s component of %s is out of range after bump", name, v)
	}
	return build(current + 1), nil
}
