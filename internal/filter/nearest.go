package filter

import (
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// AlternativeEntry is a single ranked alternative version returned by NearestVersions.
type AlternativeEntry struct {
	Version string `json:"version"`
	Reason  string `json:"reason"`
}

// Reason constants used in AlternativeEntry.Reason.
const (
	ReasonLatestStable  = "latest_stable"
	ReasonNearestSemver = "nearest_semver"
	ReasonLatestInMajor = "latest_in_major"
)

// NearestVersions returns a ranked, deduplicated slice of at most 5 alternative
// versions relative to target. The result always starts with latestStable and
// then adds nearest_semver and latest_in_major candidates.
//
// Ranking uses 3-tier distance:
//  1. Same minor as target → ranked by patch distance (lowest first)
//  2. Same major, different minor → ranked by minor distance (lowest first)
//  3. Different major → ranked by major distance (lowest first)
//
// Higher version wins when distances are tied.
//
// Parameters:
//   - versions: raw version strings from the registry.
//   - target: the requested (but missing) version.
//   - vPrefixed: when true, versions and target already carry a "v" prefix.
//   - latestStable: the overall latest stable version (always first in result).
//
// Returns nil when versions is empty or latestStable is empty.
// If target is not valid semver (e.g. a Go pseudo-version or a PEP 440 epoch
// version like "1!2.0.0"), distance ranking is skipped and the result contains
// only latestStable with reason "latest_stable".
func NearestVersions(versions []string, target string, vPrefixed bool, latestStable string) []AlternativeEntry {
	if len(versions) == 0 || latestStable == "" {
		return nil
	}

	// Normalise target for semver operations.
	normTarget := target
	if !vPrefixed {
		normTarget = VPrefix(target)
	}
	if !semver.IsValid(normTarget) {
		// Cannot rank distances; return just latestStable.
		return []AlternativeEntry{{Version: latestStable, Reason: ReasonLatestStable}}
	}

	// Build filtered candidate list (valid, non-prerelease, non-pseudo).
	// Stored with "v" prefix for semver comparisons.
	var candidates []string
	for _, raw := range versions {
		v := raw
		if !vPrefixed {
			v = VPrefix(raw)
		}
		if !semver.IsValid(v) {
			continue
		}
		if semver.Prerelease(v) != "" {
			continue
		}
		if vPrefixed && module.IsPseudoVersion(v) {
			continue
		}
		candidates = append(candidates, v)
	}

	// Helper: convert v-normalised string back to caller's convention.
	toOut := func(v string) string {
		if vPrefixed {
			return v
		}
		return StripV(v)
	}

	// Normalise latestStable to v-prefixed form for comparisons.
	normLatest := latestStable
	if !vPrefixed {
		normLatest = VPrefix(latestStable)
	}

	targetMajor := semver.Major(normTarget)
	targetMajMin := semver.MajorMinor(normTarget)

	// Result starts with latestStable; track what is already included.
	result := []AlternativeEntry{{Version: latestStable, Reason: ReasonLatestStable}}
	inResult := map[string]bool{normLatest: true}

	// ---- nearest_semver ----
	// Sort candidates by 3-tier distance from target, preferring higher on tie.
	type ranked struct {
		v     string
		tier  int // 1=same minor, 2=same major, 3=different major
		dist  int // primary distance within tier
	}
	var byDist []ranked
	for _, v := range candidates {
		if inResult[v] {
			continue
		}
		vMajor := semver.Major(v)
		vMajMin := semver.MajorMinor(v)

		var tier, dist int
		switch {
		case vMajMin == targetMajMin:
			// Same minor — rank by patch distance.
			_, _, tPatch := parseParts(normTarget)
			_, _, vPatch := parseParts(v)
			tier = 1
			dist = abs(tPatch - vPatch)
		case vMajor == targetMajor:
			// Same major, different minor — rank by minor distance.
			_, tMinor, _ := parseParts(normTarget)
			_, vMinor, _ := parseParts(v)
			tier = 2
			dist = abs(tMinor - vMinor)
		default:
			// Different major — rank by major distance.
			tMajor, _, _ := parseParts(normTarget)
			vMajorN, _, _ := parseParts(v)
			tier = 3
			dist = abs(tMajor - vMajorN)
		}
		byDist = append(byDist, ranked{v, tier, dist})
	}

	sort.SliceStable(byDist, func(i, j int) bool {
		a, b := byDist[i], byDist[j]
		if a.tier != b.tier {
			return a.tier < b.tier
		}
		if a.dist != b.dist {
			return a.dist < b.dist
		}
		// Tie: prefer higher version.
		return semver.Compare(a.v, b.v) > 0
	})

	if len(byDist) > 0 {
		nearest := byDist[0].v
		result = append(result, AlternativeEntry{
			Version: toOut(nearest),
			Reason:  ReasonNearestSemver,
		})
		inResult[nearest] = true
	}

	// ---- latest_in_major ----
	// Find the highest version within target's own major (excluding latestStable).
	// Only add it if the best candidate isn't already in the result — we don't
	// fall back to the second-highest (that would be noise, not signal).
	var highestInMajor string
	for _, v := range candidates {
		if v == normLatest {
			continue
		}
		if semver.Major(v) != targetMajor {
			continue
		}
		if highestInMajor == "" || semver.Compare(v, highestInMajor) > 0 {
			highestInMajor = v
		}
	}
	// Add latest_in_major only when the overall best for the major is not yet
	// in the result.
	latestInMajorNorm := ""
	if highestInMajor != "" && !inResult[highestInMajor] {
		latestInMajorNorm = highestInMajor
	}
	if latestInMajorNorm != "" {
		result = append(result, AlternativeEntry{
			Version: toOut(latestInMajorNorm),
			Reason:  ReasonLatestInMajor,
		})
		inResult[latestInMajorNorm] = true
	}

	return result
}

// parseParts extracts the major, minor, and patch integers from a valid semver
// string (with "v" prefix).
func parseParts(v string) (major, minor, patch int) {
	maj := semver.Major(v)         // e.g. "v1"
	majMin := semver.MajorMinor(v) // e.g. "v1.2" or "" on invalid

	majStr := strings.TrimPrefix(maj, "v")
	major, _ = strconv.Atoi(majStr)

	if majMin != "" {
		rest := strings.TrimPrefix(majMin, maj+".")
		minor, _ = strconv.Atoi(rest)
	}

	// Patch: strip "vMAJOR.MINOR." prefix, take numeric part before any suffix.
	if majMin != "" {
		patchPart := strings.TrimPrefix(v, majMin+".")
		dotIdx := strings.IndexAny(patchPart, "-+")
		if dotIdx >= 0 {
			patchPart = patchPart[:dotIdx]
		}
		patch, _ = strconv.Atoi(patchPart)
	}
	return major, minor, patch
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
