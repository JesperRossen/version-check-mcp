// Package gh builds URLs for the GitHub REST API and houses the GitHub Actions
// registry adapter. This file contains the URL builders only; the adapter type
// and HTTP wiring live in gh.go.
//
// The host is hardcoded to api.github.com to mitigate SSRF (T-03-gh-02):
// there is no caller-supplied base URL. The "owner/repo" path segment is
// concatenated directly — the MCP handler validates the package-name shape
// (must contain exactly one '/') before calling the adapter.
package gh

import "fmt"

// tagsURL returns the GitHub API URL for the paginated tags list of a
// repository. per_page is fixed at 100 (the maximum) to minimise round-trips.
// D-GH-01: up to two pages are fetched; page is 1 or 2.
func tagsURL(repo string, page int) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/tags?per_page=100&page=%d", repo, page)
}

// releasesLatestURL returns the GitHub API URL for the latest release of a
// repository. Used as the latest-stable hint (D-GH-02, source=
// "registry-release-pointer") when no major/minor filter is applied and
// incPre=false.
func releasesLatestURL(repo string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
}
