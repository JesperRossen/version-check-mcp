package filter

import (
	"regexp"
	"strconv"
	"strings"
)

// pre-compiled regexps for PEP 440 normalization rules.
// See peps.python.org/pep-0440/#normalization for the canonical spec.

// Rule 3+4: optional separator ([-_.]) followed by a pre-release label and optional digits.
// Longer alternatives listed first to avoid prefix ambiguity (e.g. "alpha" before "a").
var reSepPreLabel = regexp.MustCompile(`[-_.]?(alpha|beta|preview|pre|rc|a|b|c)(\d*)`)

// Rule 5: separators before post/dev must be a dot. Replace [-_] with ".".
var reSepPost = regexp.MustCompile(`[-_](post|dev)(\d*)`)

// PEP440Normalize normalises a version string to its canonical PEP 440 form.
//
// Rules applied (peps.python.org/pep-0440/#normalization):
//  1. TrimSpace.
//  2. Lowercase.
//  3. Strip leading "v".
//  4. Alias pre-release labels: alpha→a, beta→b, c/preview/pre→rc.
//  5. Remove separator before pre-release label (a, b, rc).
//  6. Ensure single "." separator before "post" and "dev".
//  7. Append "0" when a pre-release label has no trailing number (e.g. "1.0a" → "1.0a0").
//  8. Strip leading zeros from numeric segments.
func PEP440Normalize(v string) string {
	if v == "" {
		return v
	}

	// Rule 1: TrimSpace + Rule 2: lowercase.
	v = strings.ToLower(strings.TrimSpace(v))

	// Rule 3 (part): strip leading "v".
	v = strings.TrimPrefix(v, "v")

	// Apply pre-release label normalization as a single pass.
	// The regex matches an optional separator followed by a label and optional digits.
	// Longer alternatives come first in the regex so "alpha" wins over "a", etc.
	v = reSepPreLabel.ReplaceAllStringFunc(v, func(m string) string {
		parts := reSepPreLabel.FindStringSubmatch(m)
		// The outer match guarantees len(parts)==3; assert to catch future regex edits.
		if len(parts) != 3 {
			panic("pep440: regex subgroup count changed; fix reSepPreLabel")
		}
		label := parts[1]
		digits := parts[2]

		// Rule 4: alias labels to canonical forms.
		switch label {
		case "alpha":
			label = "a"
		case "beta":
			label = "b"
		case "c", "preview", "pre":
			label = "rc"
		// "a", "b", "rc" remain unchanged.
		}

		// Rule 7: implicit trailing zero when no digits follow the label.
		if digits == "" {
			digits = "0"
		}

		return label + digits
	})

	// Rule 5: ensure "." separator before post/dev (replace [-_] with ".").
	v = reSepPost.ReplaceAllString(v, ".$1$2")

	// Rule 8: strip leading zeros from dot-separated numeric segments.
	// Only pure-numeric segments are normalised (pre-release labels are untouched).
	parts := strings.Split(v, ".")
	for i, p := range parts {
		if n, err := strconv.Atoi(p); err == nil {
			parts[i] = strconv.Itoa(n)
		}
	}
	v = strings.Join(parts, ".")

	return v
}
