// Package maven provides a registry adapter for Maven Central via
// repo1.maven.org. The URL builder in this file handles the group-dot-to-path-
// slash conversion required by the maven-metadata.xml endpoint.
package maven

import "strings"

// MetadataURL builds the maven-metadata.xml URL for the given group and
// artifact. Group IDs use dot notation (e.g. "org.springframework"); this
// function converts them to path notation (e.g. "org/springframework") per
// D-MAVEN-01.
//
// The host is hardcoded to repo1.maven.org/maven2 — the canonical Maven
// Central CDN. No user-controlled input reaches the host portion (T-03-maven-03).
func MetadataURL(group, artifact string) string {
	groupPath := strings.ReplaceAll(group, ".", "/")
	return "https://repo1.maven.org/maven2/" + groupPath + "/" + artifact + "/maven-metadata.xml"
}
