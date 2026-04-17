package handler

import (
	"fmt"
	"strings"

	"github.com/pypx/api/internal/pypi"
)

// stubRegistry maps lowercase PyPI package names to their known type stub packages.
// Only add entries for verified, well-maintained stub packages.
var stubRegistry = map[string]string{
	"django":              "django-stubs",
	"djangorestframework": "djangorestframework-stubs",
	"requests":            "types-requests",
	"boto3":               "boto3-stubs",
	"pandas":              "pandas-stubs",
	"pillow":              "types-Pillow",
	"redis":               "types-redis",
	"pyyaml":              "types-PyYAML",
	"stripe":              "types-stripe",
}

// lookupStubPackage returns the stub package name for a source package, if known.
func lookupStubPackage(name string) (string, bool) {
	stub, ok := stubRegistry[strings.ToLower(name)]
	return stub, ok
}

// resolveStubVersion picks the best stub version for the given source major version.
// It prefers the latest stub release whose major version matches; falls back to the
// latest overall release if no major-version match exists.
func resolveStubVersion(releases map[string][]pypi.ReleaseFile, sourceMajor string) string {
	var matching, all []string
	for v := range releases {
		all = append(all, v)
		if strings.HasPrefix(v, sourceMajor+".") || v == sourceMajor {
			matching = append(matching, v)
		}
	}
	if len(matching) > 0 {
		return latestVersion(matching)
	}
	return latestVersion(all)
}

// latestVersion returns the highest version from versions by numeric segment comparison.
func latestVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	best := versions[0]
	for _, v := range versions[1:] {
		if compareVersions(v, best) > 0 {
			best = v
		}
	}
	return best
}

// compareVersions compares two version strings numerically segment by segment.
// Returns 1 if a > b, -1 if a < b, 0 if equal.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	n := len(aParts)
	if len(bParts) > n {
		n = len(bParts)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &av) //nolint:errcheck
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bv) //nolint:errcheck
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}
