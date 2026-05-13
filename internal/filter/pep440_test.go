package filter_test

import (
	"testing"

	"github.com/JesperRossen/version-check-mcp/internal/filter"
)

func TestPEP440Normalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		// Rule 1+2: lowercase + strip leading v
		{name: "uppercase V stripped and lowercased", in: "V1.0.0", want: "1.0.0"},
		{name: "lowercase v stripped", in: "v1.0.0", want: "1.0.0"},

		// Rule 3: strip separator before pre-release label
		{name: "dash before rc stripped", in: "1.0.0-rc1", want: "1.0.0rc1"},
		{name: "underscore before rc stripped", in: "1.0.0_rc1", want: "1.0.0rc1"},
		{name: "dot before rc stripped", in: "1.0.0.rc1", want: "1.0.0rc1"},
		{name: "dash before alpha stripped", in: "1.0.0-alpha1", want: "1.0.0a1"},
		{name: "dash before beta stripped", in: "1.0.0-beta2", want: "1.0.0b2"},

		// Rule 4: alias pre-release labels
		{name: "alpha aliased to a", in: "1.0.0alpha1", want: "1.0.0a1"},
		{name: "beta aliased to b", in: "1.0.0beta2", want: "1.0.0b2"},
		{name: "c aliased to rc", in: "1.0.0c1", want: "1.0.0rc1"},
		{name: "preview aliased to rc", in: "1.0.0preview1", want: "1.0.0rc1"},
		{name: "pre aliased to rc", in: "1.0.0pre1", want: "1.0.0rc1"},

		// Rule 5: ensure dot separator before post/dev
		{name: "dash-post becomes dot-post", in: "1.0-post1", want: "1.0.post1"},
		{name: "dash-dev becomes dot-dev", in: "1.0-dev1", want: "1.0.dev1"},
		{name: "underscore-post becomes dot-post", in: "1.0_post1", want: "1.0.post1"},

		// Rule 6: implicit trailing zero on pre-release label
		{name: "a with no number gets 0", in: "1.0a", want: "1.0a0"},
		{name: "b with no number gets 0", in: "1.0b", want: "1.0b0"},
		{name: "rc with no number gets 0", in: "1.0rc", want: "1.0rc0"},

		// Rule 7: strip leading zeros in numeric segments
		{name: "leading zeros stripped from segments", in: "01.02.03", want: "1.2.3"},
		{name: "leading zero in single segment", in: "01.0.0", want: "1.0.0"},

		// TrimSpace
		{name: "whitespace trimmed", in: "  1.0.0  ", want: "1.0.0"},

		// Idempotent on canonical forms
		{name: "canonical release is idempotent", in: "2.31.0rc1", want: "2.31.0rc1"},
		{name: "canonical release with numbers", in: "1.0.0", want: "1.0.0"},

		// Edge cases
		{name: "empty string is idempotent", in: "", want: ""},
		{name: "already normalized a", in: "1.0a1", want: "1.0a1"},
		{name: "already normalized b", in: "1.0b2", want: "1.0b2"},
		{name: "post with dot already", in: "1.0.post1", want: "1.0.post1"},
		{name: "dev with dot already", in: "1.0.dev0", want: "1.0.dev0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filter.PEP440Normalize(tc.in)
			if got != tc.want {
				t.Errorf("PEP440Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
