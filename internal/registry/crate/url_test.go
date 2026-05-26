package crate

import "testing"

func TestCrateURL_EscapesPathSegment(t *testing.T) {
	got := crateURL("serde?name=#beta%2F")
	want := "https://crates.io/api/v1/crates/serde%3Fname=%23beta%252F"
	if got != want {
		t.Fatalf("crateURL() = %q, want %q", got, want)
	}
}
