package gh

import (
	"strings"
	"testing"
)

func TestURL_Tags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		repo string
		page int
		want string
	}{
		{
			repo: "actions/checkout",
			page: 1,
			want: "https://api.github.com/repos/actions/checkout/tags?per_page=100&page=1",
		},
		{
			repo: "actions/checkout",
			page: 2,
			want: "https://api.github.com/repos/actions/checkout/tags?per_page=100&page=2",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got := tagsURL(tc.repo, tc.page)
			if got != tc.want {
				t.Fatalf("tagsURL(%q, %d) = %q, want %q", tc.repo, tc.page, got, tc.want)
			}
			if !strings.HasPrefix(got, "https://api.github.com/") {
				t.Fatalf("tagsURL host not api.github.com: %q", got)
			}
		})
	}
}

func TestURL_ReleasesLatest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		repo string
		want string
	}{
		{
			repo: "actions/checkout",
			want: "https://api.github.com/repos/actions/checkout/releases/latest",
		},
		{
			repo: "actions/setup-go",
			want: "https://api.github.com/repos/actions/setup-go/releases/latest",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got := releasesLatestURL(tc.repo)
			if got != tc.want {
				t.Fatalf("releasesLatestURL(%q) = %q, want %q", tc.repo, got, tc.want)
			}
			if !strings.HasPrefix(got, "https://api.github.com/") {
				t.Fatalf("releasesLatestURL host not api.github.com: %q", got)
			}
		})
	}
}
