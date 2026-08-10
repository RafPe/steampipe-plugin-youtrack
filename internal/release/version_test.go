package release

import (
	"strings"
	"testing"
)

func TestParseSemVer(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in      string
		want    SemVer
		wantErr string
	}{
		"zero version":       {in: "v0.0.0", want: SemVer{0, 0, 0}},
		"typical":            {in: "v1.2.3", want: SemVer{1, 2, 3}},
		"pre-1.0":            {in: "v0.1.0", want: SemVer{0, 1, 0}},
		"multi-digit":        {in: "v10.20.30", want: SemVer{10, 20, 30}},
		"zero component":     {in: "v1.0.10", want: SemVer{1, 0, 10}},
		"missing v prefix":   {in: "1.2.3", wantErr: "prefix"},
		"uppercase V prefix": {in: "V1.2.3", wantErr: "prefix"},
		"prerelease suffix":  {in: "v1.0.0-rc1", wantErr: "invalid"},
		"build suffix":       {in: "v1.0.0+build1", wantErr: "invalid"},
		"leading zero major": {in: "v01.0.0", wantErr: "leading zero"},
		"leading zero minor": {in: "v1.01.0", wantErr: "leading zero"},
		"leading zero patch": {in: "v1.0.01", wantErr: "leading zero"},
		"too few components": {in: "v1.2", wantErr: "invalid"},
		"too many components": {
			in: "v1.2.3.4", wantErr: "invalid",
		},
		"empty":           {in: "", wantErr: "prefix"},
		"just v":          {in: "v", wantErr: "invalid"},
		"garbage":         {in: "vX.Y.Z", wantErr: "invalid"},
		"negative major":  {in: "v-1.0.0", wantErr: "invalid"},
		"overflow major":  {in: "v2147483648.0.0", wantErr: "range"},
		"overflow minor":  {in: "v0.2147483648.0", wantErr: "range"},
		"overflow patch":  {in: "v0.0.2147483648", wantErr: "range"},
		"max int32 is ok": {in: "v2147483647.0.0", want: SemVer{2147483647, 0, 0}},
		"trailing dot":    {in: "v1.2.3.", wantErr: "invalid"},
		"empty component": {in: "v1..3", wantErr: "invalid"},
		"whitespace around": {
			in: " v1.2.3", wantErr: "prefix",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseSemVer(tt.in)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ParseSemVer(%q) error = %v, want nil", tt.in, err)
				}
				if got != tt.want {
					t.Errorf("ParseSemVer(%q) = %+v, want %+v", tt.in, got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("ParseSemVer(%q) = %+v, want error containing %q", tt.in, got, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseSemVer(%q) error = %v, want containing %q", tt.in, err, tt.wantErr)
			}
		})
	}
}

func TestSemVerString(t *testing.T) {
	t.Parallel()

	got := SemVer{Major: 1, Minor: 2, Patch: 3}.String()
	if got != "v1.2.3" {
		t.Errorf("String() = %q, want %q", got, "v1.2.3")
	}
}

func TestSemVerBump(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in      SemVer
		kind    string
		want    SemVer
		wantErr string
	}{
		"pre-1.0 major":   {in: SemVer{0, 1, 0}, kind: "major", want: SemVer{1, 0, 0}},
		"pre-1.0 minor":   {in: SemVer{0, 1, 0}, kind: "minor", want: SemVer{0, 2, 0}},
		"pre-1.0 patch":   {in: SemVer{0, 1, 0}, kind: "patch", want: SemVer{0, 1, 1}},
		"post-1.0 major":  {in: SemVer{1, 4, 7}, kind: "major", want: SemVer{2, 0, 0}},
		"post-1.0 minor":  {in: SemVer{1, 4, 7}, kind: "minor", want: SemVer{1, 5, 0}},
		"post-1.0 patch":  {in: SemVer{1, 4, 7}, kind: "patch", want: SemVer{1, 4, 8}},
		"bootstrap minor": {in: SemVer{0, 0, 0}, kind: "minor", want: SemVer{0, 1, 0}},
		"invalid kind":    {in: SemVer{1, 0, 0}, kind: "bogus", wantErr: "invalid bump kind"},
		"major overflow":  {in: SemVer{2147483647, 0, 0}, kind: "major", wantErr: "range"},
		"minor overflow":  {in: SemVer{0, 2147483647, 0}, kind: "minor", wantErr: "range"},
		"patch overflow":  {in: SemVer{0, 0, 2147483647}, kind: "patch", wantErr: "range"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.in.Bump(tt.kind)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Bump(%q) error = %v, want nil", tt.kind, err)
				}
				if got != tt.want {
					t.Errorf("Bump(%q) = %+v, want %+v", tt.kind, got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("Bump(%q) = %+v, want error containing %q", tt.kind, got, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Bump(%q) error = %v, want containing %q", tt.kind, err, tt.wantErr)
			}
		})
	}
}
