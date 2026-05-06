// version_test.go
package main

import "testing"

func TestPEP440Normalize(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// Already PEP 440 — pass through.
		{"1.2.3", "1.2.3"},
		{"1.2.3a1", "1.2.3a1"},
		{"1.2.3rc2", "1.2.3rc2"},
		{"1.2.3b10", "1.2.3b10"},

		// Hyphenated alpha/beta/rc with no counter → counter defaults to 0.
		{"1.2.3-alpha", "1.2.3a0"},
		{"1.2.3-beta", "1.2.3b0"},
		{"1.2.3-rc", "1.2.3rc0"},
		{"1.2.3-pre", "1.2.3a0"},

		// Hyphen + dot + number.
		{"1.2.3-alpha.1", "1.2.3a1"},
		{"1.2.3-beta.2", "1.2.3b2"},
		{"1.2.3-rc.3", "1.2.3rc3"},

		// Hyphen + number (no dot).
		{"1.2.3-alpha1", "1.2.3a1"},
		{"1.2.3-beta2", "1.2.3b2"},
		{"1.2.3-rc1", "1.2.3rc1"},

		// Case-insensitive.
		{"1.2.3-Alpha.1", "1.2.3a1"},
		{"1.2.3-RC2", "1.2.3rc2"},
		{"1.2.3-BETA", "1.2.3b0"},

		// Empty input.
		{"", ""},

		// Unrecognised suffixes are left alone.
		{"1.2.3-foo", "1.2.3-foo"},
		{"1.2.3+build.5", "1.2.3+build.5"},
	}

	for _, c := range cases {
		got := pep440Normalize(c.in)
		if got != c.want {
			t.Errorf("pep440Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
