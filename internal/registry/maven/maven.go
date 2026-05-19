// Package maven provides a registry.Registry adapter for Maven Central.
// It fetches maven-metadata.xml from repo1.maven.org, parses it with
// encoding/xml (stdlib), and applies the SNAPSHOT and release-pointer logic
// per D-MAVEN-01..05.
//
// Security (T-03-maven-01): parsePkg enforces strict "exactly one colon, both
// segments non-empty" before any URL construction.
// Security (T-03-maven-02): encoding/xml stdlib handles malformed input safely;
// HTTP client Timeout bounds response body size.
// Security (T-03-maven-03): MetadataURL hardcodes the repo1.maven.org host.
package maven

import (
	"context"
	"encoding/xml"
	"net/http"
	"strings"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/filter"
	"github.com/JesperRossen/version-check-mcp/internal/httperr"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
)

// Compile-time assertion: Adapter must implement registry.Registry.
var _ registry.Registry = (*Adapter)(nil)

// Source enum constants used by the Maven adapter (D-SOURCE-ENUM).
const (
	sourceVersionsList      = "versions-list"          // membership in <versions>
	sourceReleasePointer    = "registry-release-pointer" // <release> element trusted verbatim
	sourceComputedHighest   = "computed-highest"         // filtered highest from <versions>
)

// mavenMetadata is the top-level structure of maven-metadata.xml.
// Source: live repo1.maven.org/maven2/org/springframework/spring-core/maven-metadata.xml
type mavenMetadata struct {
	XMLName    xml.Name        `xml:"metadata"`
	GroupID    string          `xml:"groupId"`
	ArtifactID string          `xml:"artifactId"`
	Versioning mavenVersioning `xml:"versioning"`
}

// mavenVersioning holds the versioning sub-element.
type mavenVersioning struct {
	Latest      string   `xml:"latest"`
	Release     string   `xml:"release"`
	Versions    []string `xml:"versions>version"`
	LastUpdated string   `xml:"lastUpdated"`
}

// Adapter implements registry.Registry against repo1.maven.org.
type Adapter struct {
	client *http.Client
	cache  *cache.Cache
}

// New constructs a Maven Adapter. The client should be the shared *http.Client
// with UA-injecting transport; c is the shared *cache.Cache.
func New(client *http.Client, c *cache.Cache) *Adapter {
	return &Adapter{client: client, cache: c}
}

// Name returns "maven".
func (a *Adapter) Name() string { return "maven" }

// parsePkg splits "group:artifact" into its components. Returns KindInvalidInput
// if the format is not exactly "non-empty:non-empty" (D-MAVEN-05, T-03-maven-01).
//
// SplitN limit=2 means ["group", "remainder"]; any extra colons stay in
// remainder, which is then caught by the strings.Contains(parts[1], ":") guard.
func parsePkg(pkg string) (group, artifact string, err error) {
	parts := strings.SplitN(pkg, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(parts[1], ":") {
		return "", "", errs.InvalidInput(
			"maven package must be in group:artifact format",
			"pkg", pkg,
		)
	}
	return parts[0], parts[1], nil
}

// metadataFor fetches and caches the maven-metadata.xml for the given package.
// incPre is included in the cache key so that Latest(incPre=true) and
// Latest(incPre=false) do not share a slot (D-08 / CACHE-02).
func (a *Adapter) metadataFor(ctx context.Context, pkg string, incPre bool) (*mavenMetadata, error) {
	group, artifact, err := parsePkg(pkg)
	if err != nil {
		return nil, err
	}
	key := cache.Key{Manager: "maven", Pkg: pkg, Op: "metadata", IncPre: incPre}
	return cache.Get[*mavenMetadata](ctx, a.cache, key, func(ctx context.Context) (*mavenMetadata, error) {
		url := MetadataURL(group, artifact)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg)
		}
		req.Header.Set("Accept", "application/xml")
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, httperr.MapHTTPStatus(resp, pkg, "maven")
		}
		var m mavenMetadata
		if err := xml.NewDecoder(resp.Body).Decode(&m); err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg, "reason", "malformed_body")
		}
		return &m, nil
	})
}

// Validate answers "does this exact version exist for this package?". It checks
// membership in the <versions> list — it is an existence question, not a
// stability filter, so SNAPSHOTs in <versions> return Exists:true when queried
// explicitly (D-FILTER-01).
func (a *Adapter) Validate(ctx context.Context, pkg, version string, incPre bool) (registry.ValidateResult, error) {
	m, err := a.metadataFor(ctx, pkg, incPre)
	if err != nil {
		return registry.ValidateResult{}, err
	}
	for _, v := range m.Versioning.Versions {
		if v == version {
			return registry.ValidateResult{Exists: true, Source: sourceVersionsList}, nil
		}
	}
	return registry.ValidateResult{}, errs.NotFound(
		"maven version not in versions list",
		"pkg", pkg,
		"version", version,
	)
}

// Latest answers "what is the latest version?".
//
// Fast path (D-MAVEN-03): when incPre=false and no major/minor filter is set,
// trust the <release> element directly. Source: "registry-release-pointer".
//
// Fallback: compute the highest from <versions> using filter.FilterAndPickHighest.
// When incPre=false, SNAPSHOT versions are also explicitly filtered out
// (D-MAVEN-04) before the call. Source: "computed-highest".
//
// The <latest> element is deliberately ignored — it often points to a SNAPSHOT
// which would violate the stable-latest contract (D-MAVEN-03 pitfall).
func (a *Adapter) Latest(ctx context.Context, pkg string, incPre bool, major, minor *int) (registry.LatestResult, error) {
	m, err := a.metadataFor(ctx, pkg, incPre)
	if err != nil {
		return registry.LatestResult{}, err
	}

	// Fast path: stable-only, no filter — trust the <release> pointer.
	if !incPre && major == nil && minor == nil && m.Versioning.Release != "" {
		return registry.LatestResult{
			Version: m.Versioning.Release,
			Source:  sourceReleasePointer,
		}, nil
	}

	// Fallback: compute highest from <versions>.
	// When stable-only, explicitly remove SNAPSHOTs before passing to the
	// filter (D-MAVEN-04). filter.FilterAndPickHighest also removes versions
	// with a semver prerelease segment, but "-SNAPSHOT" is Maven-conventional
	// and not semver — the explicit HasSuffix check is necessary.
	candidates := make([]string, 0, len(m.Versioning.Versions))
	for _, v := range m.Versioning.Versions {
		if !incPre && strings.HasSuffix(v, "-SNAPSHOT") {
			continue
		}
		candidates = append(candidates, v)
	}

	// vPrefixed=false: Maven versions have no "v" prefix.
	winner, ok := filter.FilterAndPickHighest(candidates, false, incPre, major, minor)
	if !ok {
		return registry.LatestResult{}, errs.NotFound(
			"no version matches filter",
			"pkg", pkg, "incPre", incPre, "major", major, "minor", minor,
		)
	}
	return registry.LatestResult{Version: winner, Source: sourceComputedHighest}, nil
}

// Versions returns all known version strings for the package. The list comes
// from the maven-metadata.xml <versions> element, which is already cached
// after the first Validate or Latest call. Versions are unprefixed
// (Maven ecosystem-native).
func (a *Adapter) Versions(ctx context.Context, pkg string, incPre bool) ([]string, error) {
	meta, err := a.metadataFor(ctx, pkg, incPre)
	if err != nil {
		return nil, err
	}
	return meta.Versioning.Versions, nil
}
