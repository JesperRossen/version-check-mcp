package filter_test

import (
	"testing"

	"github.com/JesperRossen/version-check-mcp/internal/filter"
)

func ptr(n int) *int { return &n }

func TestFilterAndPickHighest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		versions  []string
		vPrefixed bool
		incPre    bool
		major     *int
		minor     *int
		wantVer   string
		wantOK    bool
	}{
		{
			name:      "vPrefixed=false strips v on return (NPM/PyPI/Maven path)",
			versions:  []string{"1.0.0", "2.0.0", "1.5.0"},
			vPrefixed: false,
			incPre:    false,
			wantVer:   "2.0.0",
			wantOK:    true,
		},
		{
			name:      "vPrefixed=true preserves v prefix (Go/GH path)",
			versions:  []string{"v1.0.0", "v2.0.0", "v1.5.0"},
			vPrefixed: true,
			incPre:    false,
			wantVer:   "v2.0.0",
			wantOK:    true,
		},
		{
			name:      "incPre=false skips prerelease versions",
			versions:  []string{"1.0.0", "2.0.0-alpha.1", "1.9.9"},
			vPrefixed: false,
			incPre:    false,
			wantVer:   "1.9.9",
			wantOK:    true,
		},
		{
			name:      "incPre=true includes prerelease versions",
			versions:  []string{"1.0.0", "2.0.0-alpha.1", "1.9.9"},
			vPrefixed: false,
			incPre:    true,
			wantVer:   "2.0.0-alpha.1",
			wantOK:    true,
		},
		{
			name:      "major filter keeps only matching major",
			versions:  []string{"1.0.0", "2.0.0", "17.1.0", "17.5.2"},
			vPrefixed: false,
			incPre:    false,
			major:     ptr(17),
			wantVer:   "17.5.2",
			wantOK:    true,
		},
		{
			name:      "major+minor filter further restricts",
			versions:  []string{"17.0.1", "17.1.0", "17.0.9"},
			vPrefixed: false,
			incPre:    false,
			major:     ptr(17),
			minor:     ptr(0),
			wantVer:   "17.0.9",
			wantOK:    true,
		},
		{
			name:      "pseudo-version excluded when incPre=false and vPrefixed=true",
			versions:  []string{"v1.0.0", "v0.0.0-20240115103000-abc123def456"},
			vPrefixed: true,
			incPre:    false,
			wantVer:   "v1.0.0",
			wantOK:    true,
		},
		{
			name:      "pseudo-version included when incPre=true",
			versions:  []string{"v0.0.0-20240115103000-abc123def456"},
			vPrefixed: true,
			incPre:    true,
			wantVer:   "v0.0.0-20240115103000-abc123def456",
			wantOK:    true,
		},
		{
			name:      "empty input returns not found",
			versions:  []string{},
			vPrefixed: false,
			incPre:    false,
			wantVer:   "",
			wantOK:    false,
		},
		{
			name:      "invalid semver inputs are silently skipped",
			versions:  []string{"not-a-version", "also-bad", "1.0.0"},
			vPrefixed: false,
			incPre:    false,
			wantVer:   "1.0.0",
			wantOK:    true,
		},
		{
			name:      "all versions filtered by major returns not found",
			versions:  []string{"1.0.0", "2.0.0"},
			vPrefixed: false,
			incPre:    false,
			major:     ptr(99),
			wantVer:   "",
			wantOK:    false,
		},
		{
			name:      "all versions are prerelease and incPre=false returns not found",
			versions:  []string{"1.0.0-alpha", "2.0.0-beta"},
			vPrefixed: false,
			incPre:    false,
			wantVer:   "",
			wantOK:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := filter.FilterAndPickHighest(tc.versions, tc.vPrefixed, tc.incPre, tc.major, tc.minor)
			if ok != tc.wantOK {
				t.Fatalf("FilterAndPickHighest() ok=%v, want %v", ok, tc.wantOK)
			}
			if got != tc.wantVer {
				t.Errorf("FilterAndPickHighest() = %q, want %q", got, tc.wantVer)
			}
		})
	}
}

func TestVPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"1.2.3", "v1.2.3"},
		{"v1.2.3", "v1.2.3"},
		{"", "v"},
	}
	for _, tc := range tests {
		if got := filter.VPrefix(tc.in); got != tc.want {
			t.Errorf("VPrefix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStripV(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := filter.StripV(tc.in); got != tc.want {
			t.Errorf("StripV(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
