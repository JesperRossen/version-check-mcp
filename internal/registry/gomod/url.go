// Package gomod implements a registry.Registry adapter for the Go Modules
// proxy protocol (proxy.golang.org).
package gomod

import (
	"golang.org/x/mod/module"
)

const proxyBase = "https://proxy.golang.org/"

// ListURL returns the GOPROXY @v/list URL for the given module path.
// The module path is escaped via module.EscapePath so capital letters are
// converted to the !-lowercase form required by the proxy protocol.
// An error is returned if module.EscapePath rejects the path.
func ListURL(mod string) (string, error) {
	escaped, err := module.EscapePath(mod)
	if err != nil {
		return "", err
	}
	return proxyBase + escaped + "/@v/list", nil
}

// LatestURL returns the GOPROXY @latest URL for the given module path.
// The module path is escaped via module.EscapePath so capital letters are
// converted to the !-lowercase form required by the proxy protocol.
// An error is returned if module.EscapePath rejects the path.
func LatestURL(mod string) (string, error) {
	escaped, err := module.EscapePath(mod)
	if err != nil {
		return "", err
	}
	return proxyBase + escaped + "/@latest", nil
}
