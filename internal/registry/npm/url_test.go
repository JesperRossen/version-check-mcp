package npm

import (
	"strings"
	"testing"
)

func TestEscapeNPMPkg(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"unscoped", "react", "react"},
		{"scoped", "@types/node", "@types%2Fnode"},
		{"scoped with extra slash — only first / escaped", "@scope/with/extra", "@scope%2Fwith/extra"},
		{"empty", "", ""},
		{"bare at sign", "@", "@"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeNPMPkg(tc.in)
			if got != tc.want {
				t.Fatalf("escapeNPMPkg(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPackumentURL_Unscoped(t *testing.T) {
	got := packumentURL("react")
	want := "https://registry.npmjs.org/react"
	if got != want {
		t.Fatalf("packumentURL(react) = %q, want %q", got, want)
	}
}

func TestPackumentURL_Scoped(t *testing.T) {
	// TEST-02 canary: scoped pkg URL must escape the first / to %2F and
	// preserve the leading @ literally.
	got := packumentURL("@types/node")
	want := "https://registry.npmjs.org/@types%2Fnode"
	if got != want {
		t.Fatalf("packumentURL(@types/node) = %q, want %q", got, want)
	}
}

func TestPackumentURL_NeverChangesHost(t *testing.T) {
	inputs := []string{"react", "@types/node", "@scope/pkg", "", "lodash"}
	const prefix = "https://registry.npmjs.org/"
	for _, in := range inputs {
		got := packumentURL(in)
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("packumentURL(%q) = %q, must start with %q (SSRF host-pin)", in, got, prefix)
		}
	}
}
