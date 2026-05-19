package filter_test

import (
	"testing"

	"github.com/JesperRossen/version-check-mcp/internal/filter"
)

func TestNearestVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		versions      []string
		target        string
		vPrefixed     bool
		latestStable  string
		wantVersions  []string
		wantReasons   []string
	}{
		{
			name:         "basic ranking: latest_stable first, nearest_semver second, latest_in_major third",
			versions:     []string{"1.0.0", "1.2.3", "1.2.4", "1.3.0", "2.0.0"},
			target:       "1.2.5",
			vPrefixed:    false,
			latestStable: "2.0.0",
			wantVersions: []string{"2.0.0", "1.2.4", "1.3.0"},
			wantReasons:  []string{"latest_stable", "nearest_semver", "latest_in_major"},
		},
		{
			name:         "deduplication: nearest_semver and latest_in_major same as latestStable -> only one entry",
			versions:     []string{"1.0.0"},
			target:       "0.9.0",
			vPrefixed:    false,
			latestStable: "1.0.0",
			wantVersions: []string{"1.0.0"},
			wantReasons:  []string{"latest_stable"},
		},
		{
			name:         "v-prefixed mode: entries carry v prefix",
			versions:     []string{"v1.2.3", "v1.2.4", "v1.3.0", "v2.0.0"},
			target:       "v1.2.5",
			vPrefixed:    true,
			latestStable: "v2.0.0",
			wantVersions: []string{"v2.0.0", "v1.2.4", "v1.3.0"},
			wantReasons:  []string{"latest_stable", "nearest_semver", "latest_in_major"},
		},
		{
			name:         "non-semver entries silently skipped",
			versions:     []string{"1.0.0", "not-a-version", "2.0.0"},
			target:       "1.5.0",
			vPrefixed:    false,
			latestStable: "2.0.0",
			wantVersions: []string{"2.0.0", "1.0.0"},
			wantReasons:  []string{"latest_stable", "nearest_semver"},
		},
		{
			name:         "tied distances prefer higher version",
			versions:     []string{"1.0.0", "1.4.0", "1.6.0", "2.0.0"},
			target:       "1.5.0",
			vPrefixed:    false,
			latestStable: "2.0.0",
			wantVersions: []string{"2.0.0", "1.6.0"},
			wantReasons:  []string{"latest_stable", "nearest_semver"},
		},
		{
			name:         "empty version list returns nil",
			versions:     []string{},
			target:       "1.0.0",
			vPrefixed:    false,
			latestStable: "1.0.0",
			wantVersions: nil,
			wantReasons:  nil,
		},
		{
			name:         "latest_in_major scoped to requested major",
			versions:     []string{"16.14.0", "17.0.0", "18.0.0"},
			target:       "16.99.0",
			vPrefixed:    false,
			latestStable: "18.0.0",
			// 16.14.0 is same-major as target (tier 2), so it ranks above 17.0.0
			// (tier 3 / different-major). It becomes nearest_semver. latest_in_major
			// also resolves to 16.14.0 but is skipped (= nearest_semver). 17.0.0
			// is not in target's major so latest_in_major never picks it.
			// Result: just 2 entries.
			wantVersions: []string{"18.0.0", "16.14.0"},
			wantReasons:  []string{"latest_stable", "nearest_semver"},
		},
		{
			name:         "deduplication: nearest_semver same as latestStable omitted",
			versions:     []string{"2.0.0", "1.0.0"},
			target:       "1.5.0",
			vPrefixed:    false,
			latestStable: "2.0.0",
			wantVersions: []string{"2.0.0", "1.0.0"},
			wantReasons:  []string{"latest_stable", "nearest_semver"},
		},
		{
			name:         "deduplication: latest_in_major same as nearest_semver omitted",
			versions:     []string{"1.3.0", "2.0.0"},
			target:       "1.5.0",
			vPrefixed:    false,
			latestStable: "2.0.0",
			wantVersions: []string{"2.0.0", "1.3.0"},
			wantReasons:  []string{"latest_stable", "nearest_semver"},
		},
		{
			name:         "prerelease versions are skipped",
			versions:     []string{"1.0.0", "1.2.0-beta.1", "2.0.0"},
			target:       "1.1.0",
			vPrefixed:    false,
			latestStable: "2.0.0",
			wantVersions: []string{"2.0.0", "1.0.0"},
			wantReasons:  []string{"latest_stable", "nearest_semver"},
		},
		{
			name:         "all versions invalid semver -> only latestStable returned",
			versions:     []string{"not-valid", "also-bad"},
			target:       "1.0.0",
			vPrefixed:    false,
			latestStable: "1.0.0",
			wantVersions: []string{"1.0.0"},
			wantReasons:  []string{"latest_stable"},
		},
		{
			// MJ-02: invalid-semver target (e.g. PEP 440 epoch "1!2.0.0") must
			// short-circuit to latestStable only, even when rankable versions exist.
			name:         "invalid-semver target short-circuits to latestStable only",
			versions:     []string{"1.0.0", "2.0.0", "3.0.0"},
			target:       "1!2.0.0",
			vPrefixed:    false,
			latestStable: "3.0.0",
			wantVersions: []string{"3.0.0"},
			wantReasons:  []string{"latest_stable"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filter.NearestVersions(tc.versions, tc.target, tc.vPrefixed, tc.latestStable)

			if tc.wantVersions == nil {
				if len(got) != 0 {
					t.Fatalf("NearestVersions() = %v (len %d), want empty/nil", got, len(got))
				}
				return
			}

			if len(got) != len(tc.wantVersions) {
				t.Fatalf("NearestVersions() returned %d entries, want %d\ngot: %+v", len(got), len(tc.wantVersions), got)
			}

			for i, entry := range got {
				if entry.Version != tc.wantVersions[i] {
					t.Errorf("entry[%d].Version = %q, want %q", i, entry.Version, tc.wantVersions[i])
				}
				if entry.Reason != tc.wantReasons[i] {
					t.Errorf("entry[%d].Reason = %q, want %q", i, entry.Reason, tc.wantReasons[i])
				}
			}
		})
	}
}
