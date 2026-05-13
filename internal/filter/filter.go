// Package filter provides shared version-filtering utilities used by all
// registry adapters. It promotes the LAT-05 filter logic from the NPM adapter
// into a standalone package, adding per-ecosystem helpers.
//
// D-FILTER-PROMOTE: promoted from internal/registry/npm/filter.go to allow
// all Phase 3 adapters (PyPI, Go Modules, GitHub Actions, Maven) to share the
// same filter logic without duplication.
package filter

import (
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// VPrefix returns v with a "v" prefix, adding it if not already present.
// Used to normalise non-prefixed ecosystem versions (NPM, PyPI, Maven) before
// feeding them to golang.org/x/mod/semver functions which require the prefix.
func VPrefix(v string) string {
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// StripV removes a leading "v" from v, if present.
// Used to restore ecosystem-native unprefixed strings (NPM, PyPI, Maven) after
// semver comparison.
func StripV(v string) string {
	return strings.TrimPrefix(v, "v")
}

// FilterAndPickHighest applies the LAT-05 three-step filter to a list of
// version strings and returns the highest version that satisfies all
// constraints, together with an ok flag.
//
// Parameters:
//   - versions: raw version strings from the registry (v-prefixed when
//     vPrefixed=true, unprefixed otherwise).
//   - vPrefixed: when true, versions already carry a "v" prefix (Go modules,
//     GitHub tags); the returned winner also carries the prefix. When false,
//     versions are unprefixed (NPM, PyPI, Maven); a temporary "v" prefix is
//     added for semver comparison and stripped from the returned winner.
//   - incPre: when false, prerelease versions (semver.Prerelease != "") are
//     excluded. When vPrefixed=true, pseudo-versions (module.IsPseudoVersion)
//     are also excluded per D-GOMOD-03.
//   - major: when non-nil, only versions whose semver.Major matches are kept.
//   - minor: when non-nil (and major != nil), additionally constrains to the
//     given minor.
//
// Returns ("", false) when no version satisfies the constraints.
// The caller's input slice is never mutated.
func FilterAndPickHighest(versions []string, vPrefixed bool, incPre bool, major, minor *int) (string, bool) {
	candidates := make([]string, 0, len(versions))
	for _, raw := range versions {
		// Normalise for semver: ensure "v" prefix is present for comparison.
		var v string
		if vPrefixed {
			v = raw
		} else {
			v = VPrefix(raw)
		}

		if !semver.IsValid(v) {
			continue
		}

		// Step 2: Prerelease filter.
		if !incPre {
			if semver.Prerelease(v) != "" {
				continue
			}
			// D-GOMOD-03: when vPrefixed=true, also exclude pseudo-versions
			// (they are classified as prerelease for filtering purposes).
			if vPrefixed && module.IsPseudoVersion(v) {
				continue
			}
		}

		// Step 1: Major / minor constraint filter.
		if major != nil {
			want := "v" + strconv.Itoa(*major)
			if semver.Major(v) != want {
				continue
			}
			if minor != nil {
				wantMM := want + "." + strconv.Itoa(*minor)
				if semver.MajorMinor(v) != wantMM {
					continue
				}
			}
		}

		candidates = append(candidates, v)
	}

	if len(candidates) == 0 {
		return "", false
	}

	sort.Slice(candidates, func(i, j int) bool {
		return semver.Compare(candidates[i], candidates[j]) < 0
	})

	winner := candidates[len(candidates)-1]
	if !vPrefixed {
		winner = StripV(winner)
	}
	return winner, true
}
