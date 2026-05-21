# Dogfood Log - version-check-mcp v1.0.0

**Window start:** 2026-05-21
**Window target:** >=7 days of daily active use
**Agents under test:** Claude Code CLI (primary), OpenCode (secondary)
**Binary:** built from main at e8b617c

---

## Tracking

### P0 Bug Tracker

| # | Description | Status | Opened | Closed |
|---|-------------|--------|--------|--------|
| - | *(none open)* | - | - | - |

### Observations

Log notable events: rate-limit hits, not-found results, upstream-down events, unexpected outputs.

| Date | Agent | Tool called | Input | Output | Notes |
|------|-------|-------------|-------|--------|-------|
| 2026-05-21 | Claude Code CLI | validate_version | npm react@18.3.1 | exists:true | P0 live validation PASS |
| 2026-05-21 | Claude Code CLI | get_latest_version | npm lodash | 4.18.1 | dist-tags.latest source, PASS |
| 2026-05-21 | Claude Code CLI | validate_version | npm react@99.0.0 | exists:false, alternatives=[19.2.6, 19.2.5] | miss+alternatives PASS |

---

## Log

<!-- One entry per day. Copy the template block below. -->

### Day 1 - 2026-05-21

**Agent used:** Claude Code CLI
**Calls made:** 2x validate_version, 1x get_latest_version
**Any issues:** none
**Notes:** P0 live validation session. All three tool calls returned correct real-registry data. validate_version react@18.3.1 confirmed exists. get_latest_version lodash returned 4.18.1 (dist-tags.latest). validate_version react@99.0.0 returned exists:false with alternatives [latest_stable: 19.2.6, nearest_semver: 19.2.5]. No startup issues, no quarantine block. Binary loaded cleanly from .mcp.json.

---

### Day N - YYYY-MM-DD

**Agent used:** <!-- e.g. Claude Code CLI -->
**Calls made:** <!-- e.g. 3x validate_version, 1x get_latest_version -->
**Any issues:** <!-- P0/P1/none -->
**Notes:** <!-- free text -->

---

## Sign-off

- [ ] >=7 days elapsed
- [ ] Zero open P0 bugs
- [ ] Rate-limit, not-found, and upstream-down events observed in stderr logs at least once each
- [ ] Author approves v1.0.0 tag
