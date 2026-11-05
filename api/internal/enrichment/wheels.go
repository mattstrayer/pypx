package enrichment

import (
	"strings"

	"github.com/pypx/api/internal/pypi"
)

// PythonVersionInfo holds parsed requires_python constraint data.
type PythonVersionInfo struct {
	Constraint string `json:"constraint"`
	MinVersion string `json:"min_version"`
}

// ExtractInstallSize returns the smallest wheel size from the provided files.
// If no wheel files are present, it falls back to the smallest sdist size.
// Returns 0 if no files are present.
func ExtractInstallSize(files []pypi.ReleaseFile) int64 {
	var smallest int64

	// Try wheels first.
	for _, f := range files {
		if f.PackageType == "bdist_wheel" {
			if smallest == 0 || f.Size < smallest {
				smallest = f.Size
			}
		}
	}
	if smallest > 0 {
		return smallest
	}

	// Fall back to sdist.
	for _, f := range files {
		if f.PackageType == "sdist" {
			if smallest == 0 || f.Size < smallest {
				smallest = f.Size
			}
		}
	}
	return smallest
}

// ExtractModuleFormat parses the wheel filename to extract the
// python-abi-platform compatibility tag (e.g. "py3-none-any").
// Returns "platform-specific" if the wheel targets a specific platform
// (manylinux, macosx, win, musllinux, etc.).
// Returns "sdist-only" if no wheel files are present.
func ExtractModuleFormat(files []pypi.ReleaseFile) string {
	for _, f := range files {
		if f.PackageType != "bdist_wheel" {
			continue
		}
		// Wheel filename format: {name}-{version}(-{build})?-{python}-{abi}-{platform}.whl
		name := strings.TrimSuffix(f.Filename, ".whl")
		parts := strings.Split(name, "-")
		if len(parts) < 5 {
			continue
		}
		// Last three parts are python-abi-platform.
		python := parts[len(parts)-3]
		abi := parts[len(parts)-2]
		platform := parts[len(parts)-1]
		tag := python + "-" + abi + "-" + platform

		// Detect platform-specific wheels.
		if isPlatformSpecific(platform) {
			return "platform-specific"
		}
		return tag
	}
	return "sdist-only"
}

// isPlatformSpecific returns true when the platform tag indicates a binary
// distribution tied to a specific OS/architecture.
func isPlatformSpecific(platform string) bool {
	platformIndicators := []string{
		"manylinux",
		"musllinux",
		"macosx",
		"win",
		"linux",
	}
	lower := strings.ToLower(platform)
	for _, indicator := range platformIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}

// ExtractPythonVersions parses a requires_python specifier string
// (e.g. ">=3.8" or ">=3.9,<4.0") and returns the constraint as-is along
// with the minimum version. The minimum is derived from the first matching
// clause using the prefix priority ">=", "~=", "==="/"==" (with a trailing
// ".*" trimmed), or ">". "!=", "<", and "<=" never yield a minimum.
func ExtractPythonVersions(requiresPython string) PythonVersionInfo {
	info := PythonVersionInfo{
		Constraint: requiresPython,
	}

	// Split on comma to handle compound specifiers like ">=3.9,<4.0".
	clauses := strings.Split(requiresPython, ",")
	minFrom := func(prefix string) string {
		for _, clause := range clauses {
			clause = strings.TrimSpace(clause)
			if strings.HasPrefix(clause, prefix) {
				v := strings.TrimSpace(strings.TrimPrefix(clause, prefix))
				return strings.TrimSuffix(v, ".*")
			}
		}
		return ""
	}
	// Order matters: ">=" before ">", "===" before "==". "!=", "<", "<=" never match.
	for _, prefix := range []string{">=", "~=", "===", "==", ">"} {
		if v := minFrom(prefix); v != "" {
			info.MinVersion = v
			break
		}
	}

	return info
}
