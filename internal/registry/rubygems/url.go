// Package rubygems builds URLs for the public RubyGems API.
package rubygems

import "net/url"

// versionsURL returns the upstream RubyGems versions endpoint for a gem.
// The host is hard-coded to keep requests pinned to rubygems.org.
func versionsURL(gem string) string {
	return "https://rubygems.org/api/v1/versions/" + url.PathEscape(gem) + ".json"
}
