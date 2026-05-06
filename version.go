// version.go — convert common GitHub release tag formats to PEP 440.
package main

import (
	"log/slog"
	"regexp"
	"strings"
)

// preReleasePattern matches a release-tag suffix like "-alpha", "-alpha.1",
// "-alpha1", "-beta2", "-rc.3", "-pre", etc. It captures the kind and
// (optional) numeric counter so we can rebuild it in PEP 440 form.
var preReleasePattern = regexp.MustCompile(`(?i)[-._]?(alpha|beta|rc|pre|a|b|c)\.?(\d*)$`)

// pep440Kind maps the matched word to its PEP 440 short form.
var pep440Kind = map[string]string{
	"alpha": "a",
	"a":     "a",
	"pre":   "a",
	"beta":  "b",
	"b":     "b",
	"rc":    "rc",
	"c":     "rc",
}

// alreadyPEP440Pre matches a version that already ends in PEP 440 pre-release
// form, e.g. "1.2.3a1", "1.2.3rc2". Such versions are returned unchanged.
var alreadyPEP440Pre = regexp.MustCompile(`^\d+(\.\d+)*(a|b|rc)\d+$`)

// pep440Normalize converts common GoReleaser/semver pre-release tags to PEP 440
// form. It handles "-alpha[.N]", "-beta[.N]", "-rc[.N]", and "-pre[.N]" with
// optional separators. Inputs already in PEP 440 form pass through unchanged;
// anything unrecognised is returned as-is.
func pep440Normalize(v string) string {
	if v == "" || alreadyPEP440Pre.MatchString(v) {
		return v
	}

	loc := preReleasePattern.FindStringSubmatchIndex(v)
	if loc == nil {
		return v
	}

	base := v[:loc[0]]
	kindRaw := strings.ToLower(v[loc[2]:loc[3]])
	num := v[loc[4]:loc[5]]

	kind, ok := pep440Kind[kindRaw]
	if !ok {
		return v
	}
	if num == "" {
		num = "0"
	}

	out := base + kind + num
	if out != v {
		slog.Debug("pep440: normalised version", "from", v, "to", out)
	}
	return out
}
