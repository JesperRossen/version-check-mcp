package gomod_test

import (
	"strings"
	"testing"

	"github.com/JesperRossen/version-check-mcp/internal/registry/gomod"
)

// TestURL_ListNominal checks that listURL returns the expected URL for a
// plain (no-capital) module path.
func TestURL_ListNominal(t *testing.T) {
	u, err := gomod.ListURL("github.com/aws/aws-sdk-go")
	if err != nil {
		t.Fatalf("listURL nominal: %v", err)
	}
	want := "https://proxy.golang.org/github.com/aws/aws-sdk-go/@v/list"
	if u != want {
		t.Fatalf("listURL = %q, want %q", u, want)
	}
}

// TestURL_ListEscapes checks that listURL applies !-escaping for capital
// letters per the Go proxy protocol.
func TestURL_ListEscapes(t *testing.T) {
	u, err := gomod.ListURL("github.com/Azure/azure-sdk-for-go")
	if err != nil {
		t.Fatalf("listURL escape: %v", err)
	}
	if !strings.Contains(u, "!azure") {
		t.Fatalf("listURL = %q, want !azure substring (capital A escaped)", u)
	}
	if !strings.HasSuffix(u, "/@v/list") {
		t.Fatalf("listURL = %q, want /@v/list suffix", u)
	}
}

// TestURL_LatestEscapes checks that latestURL applies !-escaping and uses the
// /@latest suffix.
func TestURL_LatestEscapes(t *testing.T) {
	u, err := gomod.LatestURL("github.com/Azure/azure-sdk-for-go")
	if err != nil {
		t.Fatalf("latestURL escape: %v", err)
	}
	if !strings.Contains(u, "!azure") {
		t.Fatalf("latestURL = %q, want !azure substring", u)
	}
	if !strings.HasSuffix(u, "/@latest") {
		t.Fatalf("latestURL = %q, want /@latest suffix", u)
	}
}

// TestURL_LatestSuffix checks that latestURL uses /@latest suffix.
func TestURL_LatestSuffix(t *testing.T) {
	u, err := gomod.LatestURL("github.com/aws/aws-sdk-go")
	if err != nil {
		t.Fatalf("latestURL: %v", err)
	}
	if !strings.HasSuffix(u, "/@latest") {
		t.Fatalf("latestURL = %q, want /@latest suffix", u)
	}
}

// TestURL_EmptyInputReturnsError checks that an empty module path returns an
// error (module.EscapePath rejects invalid paths).
func TestURL_EmptyInputReturnsError(t *testing.T) {
	_, err := gomod.ListURL("")
	if err == nil {
		t.Fatal("listURL(\"\") = nil error, want non-nil")
	}
	_, err = gomod.LatestURL("")
	if err == nil {
		t.Fatal("latestURL(\"\") = nil error, want non-nil")
	}
}
