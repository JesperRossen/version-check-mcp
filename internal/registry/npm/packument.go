package npm

import (
	"encoding/json"
	"io"
)

// Packument is the subset of an NPM packument response that the adapter
// decodes. Per Pitfall #5 in 02-RESEARCH.md, per-version metadata is kept as
// json.RawMessage: Phase 2 only needs the version map's KEYS (for the filter)
// and dist-tags.latest (for the fast path). Per-version blobs (README,
// maintainers, dependencies, ...) are not decoded — saving memory and time
// on multi-MB packuments such as `@types/node` (~10 MiB).
type Packument struct {
	Name     string                     `json:"name"`
	DistTags map[string]string          `json:"dist-tags"`
	Versions map[string]json.RawMessage `json:"versions"`
}

// parsePackument decodes a packument from r. The returned error is the raw
// stdlib JSON error; the adapter wraps it in errs.UpstreamDown at the call
// site (with reason="malformed_body").
func parsePackument(r io.Reader) (*Packument, error) {
	var p Packument
	if err := json.NewDecoder(r).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}
