# testdata/

Fixture inputs for adapter tests. Conventions live here so every Phase 3
adapter follows the same shape without re-litigating.

## Layout (per D-FIX-02)

```
testdata/fixtures/<adapter>/<fixture>.json
testdata/fixtures/<adapter>/<fixture>.json.headers.json   # optional
```

`<adapter>` matches the registry package name (`npm`, `pypi`, `gomod`,
`github`, `maven`). One file per upstream URL.

## Format (per D-FIX-01)

Each `<fixture>.json` is the **literal HTTP response body** as returned by the
upstream registry. Do not pretty-print it, do not strip fields, do not
re-marshal. The bytes on disk are the bytes the fake `RoundTripper` replays.

## Optional sibling: `<fixture>.json.headers.json`

When present, overrides the default response (`200 OK`,
`Content-Type: application/json`):

```json
{"status":404,"headers":{"Content-Type":"application/json"}}
```

Used by the `nonexistent` fixture to drive the 404 path through
`internal/testfixtures.FixtureClient` without inventing a body.

## Regenerating

Once the adapter test exists (lands in 02-03), recording is one line:

```
UPDATE_FIXTURES=1 go test ./internal/registry/<adapter>/...
```

Pre-02-03 fallback — fetch directly:

```
curl -sS -H 'Accept: application/json' -H 'User-Agent: version-check-mcp/dev' \
  'https://registry.npmjs.org/react' > testdata/fixtures/npm/react.json
```

## Why no goldie / cupaloy / go-cmp

DEP-02: fixture comparison uses stdlib helpers only — `bytes.Equal`,
`encoding/json` round-trip + `reflect.DeepEqual`, or `strings.Contains` as the
case warrants. No third-party diff or snapshot library is permitted.
