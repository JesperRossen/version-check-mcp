---
phase: 04-alternatives-response-shape-hardening
plan: "02"
subsystem: registry
tags: [registry, interface, versions, adapters]
dependency_graph:
  requires: []
  provides: [Registry.Versions method, all 5 adapter implementations, Fake.Versions]
  affects: [internal/registry/registry.go, internal/registry/fake/fake.go, internal/registry/npm/npm.go, internal/registry/pypi/pypi.go, internal/registry/gomod/gomod.go, internal/registry/gh/gh.go, internal/registry/maven/maven.go]
tech_stack:
  added: []
  patterns: [interface extension, cached-fetch delegation]
key_files:
  created: []
  modified:
    - internal/registry/registry.go
    - internal/registry/fake/fake.go
    - internal/registry/npm/npm.go
    - internal/registry/pypi/pypi.go
    - internal/registry/gomod/gomod.go
    - internal/registry/gh/gh.go
    - internal/registry/maven/maven.go
decisions:
  - "Versions() delegates to existing cached internal fetch methods - no extra HTTP calls on warm cache"
  - "Maven returns meta.Versioning.Versions slice directly (already []string)"
  - "GoMod/GH delegate entirely to listFor/tagsFor respectively"
  - "NPM/PyPI extract map keys into new []string slices"
metrics:
  duration: "~5 minutes"
  completed: "2026-05-19"
  tasks_completed: 2
  files_modified: 7
---

# Phase 04 Plan 02: Versions Method on Registry Interface - Summary

**One-liner:** Added `Versions(ctx, pkg, incPre) ([]string, error)` to the Registry interface and implemented it in all 5 adapters plus Fake, delegating to each adapter's existing cached fetch method.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add Versions to Registry interface + Fake | 4657489 | registry.go, fake/fake.go |
| 2 | Implement Versions in all 5 adapters | 8909331 | npm.go, pypi.go, gomod.go, gh.go, maven.go |

## What Was Built

The `Registry` interface now has 4 methods: `Validate`, `Latest`, `Versions`, and `Name`.

Each adapter's `Versions()` implementation:
- **npm**: extracts keys from `packument.Versions` map
- **pypi**: extracts keys from `project.Releases` map
- **gomod**: delegates to `listFor()` - returns v-prefixed strings
- **gh**: delegates to `tagsFor()` - returns tag names as-is
- **maven**: returns `meta.Versioning.Versions` directly (already `[]string`)

The Fake test double gains `VersionsList`, `VersionsErr`, and `VersionsCalls` fields, plus a `Versions()` method that respects the `PanicOn="versions"` hook.

## Deviations from Plan

None - plan executed exactly as written.

## Verification

```
go build ./...     OK
go test ./internal/registry/... -count=1
  fake: PASS
  gh:   PASS
  gomod: PASS
  maven: PASS
  npm:  PASS
  pypi: PASS
go vet ./...       OK
```

## Self-Check: PASSED

- [x] `internal/registry/registry.go` - Versions method present
- [x] `internal/registry/fake/fake.go` - Versions method + fields present
- [x] All 5 adapter files - Versions method present
- [x] Commit 4657489 exists
- [x] Commit 8909331 exists
- [x] `var _ registry.Registry = (*Fake)(nil)` compile check passes (fake package builds)
- [x] `var _ registry.Registry = (*Adapter)(nil)` checks pass in gh and maven packages
