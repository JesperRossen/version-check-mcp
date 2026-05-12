// Package npm builds URLs for the public npm registry and (in later plans)
// will house the full NPM adapter. This file contains only the URL builder;
// the Adapter type and HTTP wiring land in 02-03.
package npm

import "strings"

// escapeNPMPkg encodes an NPM package name for use as the path component of a
// registry URL.
//
// Why a hand-rolled encoder instead of the stdlib URL escapers:
//
//	the stdlib path escaper leaves '/' alone (it is a path separator) and
//	therefore splits "@types/node" into two path segments and breaks the
//	request. The stdlib query escaper conversely encodes '@' to "%40", which
//	npm requires literal. Neither produces "@types%2Fnode" — only the
//	hand-rolled rule below does.
//
// The npm registry wants the leading '@' preserved literally and ONLY the
// first '/' (the scope separator) escaped to %2F. See Pitfall #1 in
// 02-RESEARCH.md and threat T-02-01 in 02-01-PLAN.md.
func escapeNPMPkg(pkg string) string {
	if strings.HasPrefix(pkg, "@") {
		return strings.Replace(pkg, "/", "%2F", 1)
	}
	return pkg
}

// packumentURL returns the upstream npm registry URL for the full packument
// of pkg. The host is hard-coded to mitigate SSRF (T-02-01): there is no
// caller-supplied base URL.
func packumentURL(pkg string) string {
	return "https://registry.npmjs.org/" + escapeNPMPkg(pkg)
}
