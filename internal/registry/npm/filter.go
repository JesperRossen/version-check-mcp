// Package npm implements the NPM registry adapter for version-check-mcp.
package npm

import (
	"sort"
	"strconv"

	"golang.org/x/mod/semver"
)

// filterAndPickHighest applies the LAT-05 three-step filter to a list of
// NPM version strings (unprefixed). It returns the chosen version
// (unprefixed) and true on success, or "" and false when no version matches.
//
// The three steps (D-FILTER-01, verbatim):
//
//  1. Filter by `major`/`minor` constraint when provided:
//     - When `major != nil`, keep versions whose `semver.Major(v) == "v{major}"`.
//     - When `minor != nil` (and `major != nil`), additionally keep versions
//     whose `semver.MajorMinor(v) == "v{major}.{minor}"`.
//  2. Filter by prerelease policy:
//     - When `incPre == false`, drop versions whose `semver.Prerelease(v)`
//     is non-empty.
//  3. Pick the highest remaining via `semver.Compare` and return it.
//
// Behaviour notes:
//   - `major: 0` is a valid filter value, NOT a missing filter. `strconv.Itoa(0)`
//     gives `"0"`, so `semver.Major(v) == "v0"` works as written.
//   - Malformed version keys (`!semver.IsValid("v"+raw)`) are silently skipped.
//     NPM packument key sets occasionally contain weird strings; the filter
//     must never crash on them.
//   - The caller's input slice is never mutated.
//   - The returned version has NO leading `v` (NPM is unprefixed on the wire).
//
// The filter does NOT validate that `minor != nil` implies `major != nil` —
// that boundary is enforced by the MCP handler before this function is called.
func filterAndPickHighest(versions []string, incPre bool, major, minor *int) (string, bool) {
	candidates := make([]string, 0, len(versions))
	for _, raw := range versions {
		v := "v" + raw
		if !semver.IsValid(v) {
			continue
		}
		if !incPre && semver.Prerelease(v) != "" {
			continue
		}
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
	return candidates[len(candidates)-1][1:], true
}
