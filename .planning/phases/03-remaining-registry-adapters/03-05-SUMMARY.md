---
phase: 03-remaining-registry-adapters
plan: "05"
subsystem: registry/maven + cmd/main
tags: [maven, xml, registry-adapter, wiring, phase-3-completion]
dependency_graph:
  requires: [03-01, 03-02, 03-03, 03-04]
  provides: [maven-adapter, main-wired-all-adapters]
  affects: [cmd/version-check-mcp/main.go, internal/registry/maven]
tech_stack:
  added: []
  patterns:
    - encoding/xml for Maven metadata parsing
    - <release> pointer trusted, <latest> ignored (SNAPSHOT avoidance)
    - group:artifact -> group/artifact path conversion in URL builder
    - SNAPSHOT explicit filter via strings.HasSuffix before filter.FilterAndPickHighest
key_files:
  created:
    - internal/registry/maven/url.go
    - internal/registry/maven/maven.go
    - internal/registry/maven/maven_test.go
    - testdata/fixtures/maven/spring-core-metadata.xml
    - testdata/fixtures/maven/snapshot-metadata.xml
    - testdata/fixtures/maven/nonexistent.xml
    - testdata/fixtures/maven/nonexistent.xml.headers.json
  modified:
    - cmd/version-check-mcp/main.go
decisions:
  - "XML struct tags inline in maven.go (not a separate metadata.go) - single file keeps the type near its decoder, no need to split for this scope"
  - "SNAPSHOT filter via strings.HasSuffix in Latest before FilterAndPickHighest call - required because -SNAPSHOT is Maven-conventional, not semver; the filter package's incPre check alone would not catch it"
  - "<latest> element deliberately ignored - it reliably points to SNAPSHOTs; only <release> is trusted for stable-latest"
  - "TestLatest_IncPreAdmitsSnapshot uses com.example:snapshot-lib (2.0.0-SNAPSHOT > 1.5.0) not spring-core (7.0.7 > 7.0.7-SNAPSHOT due to semver prerelease ordering)"
  - "main.go wiring: removed fake import, added pypi/gomod/gh/maven imports; go.mod unchanged (4 direct deps)"
metrics:
  duration: "~25 minutes"
  completed: "2026-05-15"
  tasks_completed: 3
  files_changed: 8
---

# Phase 03 Plan 05: Maven Adapter + Main Wiring Summary

**One-liner:** Maven Central adapter using encoding/xml with <release>-pointer trust, SNAPSHOT exclusion via HasSuffix, and group:artifact validation; main.go wired to all five real adapters replacing all fake stubs.

## Tasks Completed

| # | Task | Commit | Status |
|---|------|--------|--------|
| 1 | Maven fixtures (nominal, SNAPSHOT, 404) | a2f49af | Done |
| 2 | Maven URL builder + adapter (TDD RED/GREEN) | 3a3cffa (RED), e2e289c (GREEN) | Done |
| 3 | Wire real adapters into main.go | 55f21de | Done |

## Implementation Details

### Maven URL Builder (url.go)

`MetadataURL(group, artifact string) string` converts group IDs to path notation:
- `strings.ReplaceAll(group, ".", "/")` - the only transformation needed
- Host hardcoded: `https://repo1.maven.org/maven2/` - no user input reaches the host (T-03-maven-03)
- Exported so test URL maps can reference it without hardcoding strings

### XML Struct Layout

Two inline structs in `maven.go` (not a separate file):

```go
type mavenMetadata struct {
    XMLName    xml.Name        `xml:"metadata"`
    GroupID    string          `xml:"groupId"`
    ArtifactID string          `xml:"artifactId"`
    Versioning mavenVersioning `xml:"versioning"`
}

type mavenVersioning struct {
    Latest      string   `xml:"latest"`
    Release     string   `xml:"release"`
    Versions    []string `xml:"versions>version"`
    LastUpdated string   `xml:"lastUpdated"`
}
```

The `Latest` field is parsed but never used - explicitly ignored per D-MAVEN-03.

### SNAPSHOT Filter Placement

SNAPSHOT filtering happens in `Latest` before calling `filter.FilterAndPickHighest`:

```go
for _, v := range m.Versioning.Versions {
    if !incPre && strings.HasSuffix(v, "-SNAPSHOT") {
        continue
    }
    candidates = append(candidates, v)
}
```

This is required because `-SNAPSHOT` is a Maven-conventional suffix that is NOT semver-standard. The `FilterAndPickHighest` function's `incPre` gate removes semver prerelease segments (e.g. `-RC1`, `-alpha`) but Maven's SNAPSHOT convention is semantically different and must be handled explicitly (D-MAVEN-04).

### <release> vs <latest> Decision

- `<latest>`: ignored. The fixture confirms it points to `7.0.7-SNAPSHOT` while stable latest is `7.0.7`.
- `<release>`: trusted for `!incPre && major==nil && minor==nil`. Source: `"registry-release-pointer"`.
- Fallback: `filter.FilterAndPickHighest` over filtered candidates. Source: `"computed-highest"`.

### group:artifact Validation

`parsePkg` uses `strings.SplitN(pkg, ":", 3)` - the limit of 3 ensures that `a:b:c` (3 parts) is rejected while `a:b` (2 parts) passes. Empty segments are also rejected (T-03-maven-01).

### main.go Wiring Diff

Removed:
- `"github.com/JesperRossen/version-check-mcp/internal/registry/fake"` import
- Four `fake.New(...)` calls

Added:
- `"github.com/JesperRossen/version-check-mcp/internal/registry/gh"` import
- `"github.com/JesperRossen/version-check-mcp/internal/registry/gomod"` import
- `"github.com/JesperRossen/version-check-mcp/internal/registry/maven"` import
- `"github.com/JesperRossen/version-check-mcp/internal/registry/pypi"` import
- `pypi.New(sharedClient, c)`, `gomod.New(sharedClient, c)`, `gh.New(sharedClient, c)`, `maven.New(sharedClient, c)`

go.mod: unchanged (4 direct deps: go-sdk, golang-lru/v2, x/sync, x/mod). `encoding/xml` is stdlib.

## Test Coverage

12 tests in `maven_test.go`:
- `TestValidate_Hit` - membership check, version present
- `TestValidate_HitSnapshotExplicit` - SNAPSHOT in <versions> returns Exists:true (validate is not a stability filter)
- `TestValidate_MissVersion` - version absent → KindNotFound
- `TestValidate_NotFound404` - 404 response → KindNotFound
- `TestValidate_InvalidPkg` - "no-colon", "a:b:c", "", ":" → KindInvalidInput
- `TestLatest_ReleasePointer` - <release> trusted, Source="registry-release-pointer"
- `TestLatest_DoesNotTrustLatestElement` - returns 7.0.7 not 7.0.7-SNAPSHOT
- `TestLatest_SnapshotFiltered` - major filter bypasses <release>; result is not -SNAPSHOT
- `TestLatest_IncPreAdmitsSnapshot` - 2.0.0-SNAPSHOT wins over 1.5.0 when incPre=true
- `TestLatest_FilterMajor` - major=6 → 6.1.0
- `TestLatest_SnapshotMetadata_ReleasePointer` - <release> pointer used even when <versions> highest is SNAPSHOT
- `TestCache_HitOnSecondCall` - exactly 1 upstream call for 2 consecutive Validate calls

Full suite: `go test ./...` passes 190 tests across 17 packages.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed fixtureDir path calculation (off-by-one level)**
- **Found during:** Task 2, first GREEN test run
- **Issue:** Test used `filepath.Dir(thisFile)` + 4 `..` segments; actual depth from `internal/registry/maven/` to module root is 3 levels
- **Fix:** Changed 4 `..` to 3 `..` in `fixtureDir()`
- **Files modified:** `internal/registry/maven/maven_test.go`
- **Commit:** included in e2e289c

**2. [Rule 1 - Bug] Fixed TestLatest_IncPreAdmitsSnapshot expectation**
- **Found during:** Task 2, GREEN test run
- **Issue:** Test expected `7.0.7-SNAPSHOT` as highest with incPre=true from spring-core fixture, but per semver `7.0.7 > 7.0.7-SNAPSHOT` (prerelease sorts below its release). The SNAPSHOT is ADMITTED but does not win.
- **Fix:** Changed test to use `com.example:snapshot-lib` fixture where `2.0.0-SNAPSHOT > 1.5.0` (different major, SNAPSHOT correctly wins)
- **Files modified:** `internal/registry/maven/maven_test.go`
- **Commit:** included in e2e289c

**3. [Rule 1 - Bug] Fixed TestCache_HitOnSecondCall cache TTL**
- **Found during:** Task 2, GREEN test run
- **Issue:** `cache.NewCache(64, 0)` with TTL=0 means entries expire immediately; second call always misses
- **Fix:** Changed to `cache.NewCache(64, 5*time.Minute)` matching the pattern in pypi_test.go
- **Files modified:** `internal/registry/maven/maven_test.go`
- **Commit:** included in e2e289c

**4. [Rule 1 - Bug] Fixed TestLatest_SnapshotFiltered bounds panic**
- **Found during:** Task 2, first GREEN test run
- **Issue:** Manual slice bounds check `res.Version[len(res.Version)-len("-SNAPSHOT"):]` panics on short strings
- **Fix:** Replaced with `strings.HasSuffix(res.Version, "-SNAPSHOT")`
- **Files modified:** `internal/registry/maven/maven_test.go`
- **Commit:** included in e2e289c

## Known Stubs

None. All five registries serve real registry data via their respective adapters.

## Threat Flags

No new threat surface beyond what the plan's threat model covers. The new network endpoint (repo1.maven.org) and input parsing (group:artifact) are both modeled in T-03-maven-01..03.

## Self-Check: PASSED

All created files exist. All commits (a2f49af, 3a3cffa, e2e289c, 55f21de) verified in git log. `go test ./...` passes 190 tests. `go build ./...` succeeds. No fake.New in main.go.
