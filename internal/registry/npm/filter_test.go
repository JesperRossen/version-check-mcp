package npm

import (
	"reflect"
	"testing"
)

func intPtr(i int) *int { return &i }

// canonicalVersions is the synthetic NPM version corpus exercised across
// the LAT-03/LAT-04/LAT-05 tests. It mirrors the kind of mess you'd actually
// see in `versions{}` keys: multiple majors, prereleases, a `0.x` line, and
// pure-garbage entries that must be silently skipped.
var canonicalVersions = []string{
	// stable
	"17.0.0", "17.0.1", "17.0.2", "17.1.0",
	"18.0.0", "18.1.0", "18.2.0", "18.3.0", "18.3.1",
	"19.0.0",
	"0.14.0", "0.14.7", "0.15.0",
	// prereleases
	"18.3.0-rc.0", "19.0.0-rc.1", "19.0.0-beta.0", "18.0.0-alpha.1",
	// malformed — must be silently skipped.
	// NB: golang.org/x/mod accepts shortened forms (treats "v1.2" as
	// "v1.2.0", IsValid=true), so we swap the plan's "1.2" for "1.2.x"
	// which IS rejected. See 02-02-SUMMARY.md for the behavioural note.
	"not-a-version", "1.2.x", "garbage", "",
}

func TestFilter_StableOnly(t *testing.T) {
	got, ok := filterAndPickHighest(canonicalVersions, false, nil, nil)
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	if got != "19.0.0" {
		t.Fatalf("expected 19.0.0, got %q", got)
	}
}

func TestFilter_IncPre(t *testing.T) {
	t.Run("stable still wins when present", func(t *testing.T) {
		got, ok := filterAndPickHighest(canonicalVersions, true, nil, nil)
		if !ok {
			t.Fatalf("expected ok=true, got false")
		}
		// stable beats RC of the same x.y.z under the standard ordering.
		if got != "19.0.0" {
			t.Fatalf("expected 19.0.0, got %q", got)
		}
	})

	t.Run("RC observable when no stable for that x.y.z", func(t *testing.T) {
		// Drop the stable 19.0.0; keep 19.0.0-rc.1.
		trimmed := make([]string, 0, len(canonicalVersions))
		for _, v := range canonicalVersions {
			if v == "19.0.0" {
				continue
			}
			trimmed = append(trimmed, v)
		}
		got, ok := filterAndPickHighest(trimmed, true, nil, nil)
		if !ok {
			t.Fatalf("expected ok=true, got false")
		}
		if got != "19.0.0-rc.1" {
			t.Fatalf("expected 19.0.0-rc.1, got %q", got)
		}
	})
}

func TestFilter_MajorMinor(t *testing.T) {
	cases := []struct {
		name        string
		incPre      bool
		major       *int
		minor       *int
		wantVersion string
		wantOK      bool
	}{
		{name: "major=17", incPre: false, major: intPtr(17), minor: nil, wantVersion: "17.1.0", wantOK: true},
		{name: "major=17,minor=0", incPre: false, major: intPtr(17), minor: intPtr(0), wantVersion: "17.0.2", wantOK: true},
		{name: "major=18", incPre: false, major: intPtr(18), minor: nil, wantVersion: "18.3.1", wantOK: true},
		{name: "major=18 incPre", incPre: true, major: intPtr(18), minor: nil, wantVersion: "18.3.1", wantOK: true},
		{name: "major=0 valid", incPre: false, major: intPtr(0), minor: nil, wantVersion: "0.15.0", wantOK: true},
		{name: "major=0,minor=14", incPre: false, major: intPtr(0), minor: intPtr(14), wantVersion: "0.14.7", wantOK: true},
		{name: "major=99 empty", incPre: false, major: intPtr(99), minor: nil, wantVersion: "", wantOK: false},
		{name: "major=18,minor=99 empty", incPre: false, major: intPtr(18), minor: intPtr(99), wantVersion: "", wantOK: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, ok := filterAndPickHighest(canonicalVersions, tc.incPre, tc.major, tc.minor)
			if ok != tc.wantOK {
				t.Fatalf("ok mismatch: want %v, got %v (version=%q)", tc.wantOK, ok, got)
			}
			if got != tc.wantVersion {
				t.Fatalf("version mismatch: want %q, got %q", tc.wantVersion, got)
			}
		})
	}
}

func TestFilter_MalformedSkipped(t *testing.T) {
	t.Run("one valid amid garbage", func(t *testing.T) {
		input := []string{"not-a-version", "1.2.x", "", "garbage", "1.0.0"}
		got, ok := filterAndPickHighest(input, false, nil, nil)
		if !ok || got != "1.0.0" {
			t.Fatalf("expected (1.0.0,true), got (%q,%v)", got, ok)
		}
	})

	t.Run("only garbage", func(t *testing.T) {
		input := []string{"not-a-version", "1.2.x", "", "garbage"}
		got, ok := filterAndPickHighest(input, false, nil, nil)
		if ok || got != "" {
			t.Fatalf("expected (\"\",false), got (%q,%v)", got, ok)
		}
	})
}

func TestFilter_NPMUnprefixedOnReturn(t *testing.T) {
	got, ok := filterAndPickHighest(canonicalVersions, false, nil, nil)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if len(got) > 0 && got[0] == 'v' {
		t.Fatalf("returned version must not have leading 'v', got %q", got)
	}
	if got != "19.0.0" {
		t.Fatalf("expected 19.0.0, got %q", got)
	}
}

func TestFilter_DoesNotMutateInput(t *testing.T) {
	src := append([]string(nil), canonicalVersions...)
	snapshot := append([]string(nil), src...)
	_, _ = filterAndPickHighest(src, false, intPtr(18), nil)
	if !reflect.DeepEqual(src, snapshot) {
		t.Fatalf("filter mutated input slice:\n want %v\n  got %v", snapshot, src)
	}
}
