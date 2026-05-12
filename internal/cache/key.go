package cache

import (
	"fmt"
	"net/url"
)

// Key identifies a single cache entry. Per D-08:
//   - Manager is the registry identifier (e.g. "npm", "pypi", "gomod").
//   - Pkg is the ecosystem-native package name (e.g. "react", "@types/node").
//   - Op is one of "validate" | "latest" | "packument"; key.go does not enforce this.
//   - IncPre is the prerelease bit (CACHE-02): include-prereleases queries
//     must not collide with stable-only queries for the same package.
//
// major/minor filters are deliberately NOT in the key — they constrain the
// result post-lookup; filtering before cache lookup would split the cache
// uselessly.
type Key struct {
	Manager string
	Pkg     string
	Op      string
	IncPre  bool
}

// String renders Key as a deterministic, collision-free string. Each string
// field is QueryEscape'd so that special characters like "/" and "|" can
// never produce ambiguous keys across different inputs.
func (k Key) String() string {
	return fmt.Sprintf("%s|%s|%s|%t",
		url.QueryEscape(k.Manager),
		url.QueryEscape(k.Pkg),
		url.QueryEscape(k.Op),
		k.IncPre,
	)
}
