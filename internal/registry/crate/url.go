// Package crate builds URLs for the public crates.io API.
package crate

import "net/url"

// crateURL returns the upstream crates.io API URL for a crate name.
// The host is hard-coded to keep requests pinned to crates.io.
func crateURL(name string) string {
	return "https://crates.io/api/v1/crates/" + url.PathEscape(name)
}
