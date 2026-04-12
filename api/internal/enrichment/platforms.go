package enrichment

import (
	"strings"

	"github.com/pypx/api/internal/pypi"
)

// PlatformCoverage indicates which OS/architecture combinations have prebuilt
// wheels for a release. All fields are false for sdist-only packages.
type PlatformCoverage struct {
	PurePython bool `json:"pure_python"`
	LinuxX86   bool `json:"linux_x86_64"`
	LinuxARM64 bool `json:"linux_arm64"`
	MacOSX86   bool `json:"macos_x86_64"`
	MacOSARM64 bool `json:"macos_arm64"`
	Windows    bool `json:"windows_x86_64"`
	Musl       bool `json:"musl"`
}

// ExtractPlatformCoverage parses wheel filenames from the provided release
// files and returns which platforms have prebuilt wheels available.
func ExtractPlatformCoverage(files []pypi.ReleaseFile) PlatformCoverage {
	var cov PlatformCoverage
	for _, f := range files {
		if f.PackageType != "bdist_wheel" {
			continue
		}
		platform := extractPlatformTag(f.Filename)
		if platform == "" {
			continue
		}
		p := strings.ToLower(platform)
		switch {
		case p == "any":
			cov.PurePython = true
		case strings.Contains(p, "manylinux") && strings.Contains(p, "x86_64"):
			cov.LinuxX86 = true
		case strings.Contains(p, "manylinux") && strings.Contains(p, "aarch64"):
			cov.LinuxARM64 = true
		case strings.Contains(p, "musllinux"):
			cov.Musl = true
		case strings.Contains(p, "macosx") && strings.Contains(p, "arm64"):
			cov.MacOSARM64 = true
		case strings.Contains(p, "macosx") && strings.Contains(p, "universal2"):
			cov.MacOSX86 = true
			cov.MacOSARM64 = true
		case strings.Contains(p, "macosx") && strings.Contains(p, "x86_64"):
			cov.MacOSX86 = true
		case strings.Contains(p, "win_amd64") || strings.Contains(p, "win32"):
			cov.Windows = true
		}
	}
	return cov
}

// extractPlatformTag returns the platform segment of a wheel filename.
// Wheel format: {name}-{ver}(-{build})?-{python}-{abi}-{platform}.whl
func extractPlatformTag(filename string) string {
	name := strings.TrimSuffix(filename, ".whl")
	parts := strings.Split(name, "-")
	if len(parts) < 5 {
		return ""
	}
	return parts[len(parts)-1]
}
