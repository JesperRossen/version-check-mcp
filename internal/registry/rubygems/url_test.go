package rubygems

import "testing"

func TestVersionsURL_EscapesPathSegment(t *testing.T) {
	got := versionsURL("rails?name=#beta%2F")
	want := "https://rubygems.org/api/v1/versions/rails%3Fname=%23beta%252F.json"
	if got != want {
		t.Fatalf("versionsURL() = %q, want %q", got, want)
	}
}
