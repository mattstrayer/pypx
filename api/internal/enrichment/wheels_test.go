package enrichment

import (
	"testing"

	"github.com/pypx/api/internal/pypi"
)

func TestExtractInstallSize(t *testing.T) {
	files := []pypi.ReleaseFile{
		{Filename: "requests-2.31.0-py3-none-any.whl", PackageType: "bdist_wheel", Size: 62574},
		{Filename: "requests-2.31.0.tar.gz", PackageType: "sdist", Size: 110346},
	}

	got := ExtractInstallSize(files)
	if got != 62574 {
		t.Errorf("ExtractInstallSize = %d, want 62574", got)
	}
}

func TestExtractInstallSizeFallsBackToSdist(t *testing.T) {
	files := []pypi.ReleaseFile{
		{Filename: "requests-2.31.0.tar.gz", PackageType: "sdist", Size: 50000},
	}

	got := ExtractInstallSize(files)
	if got != 50000 {
		t.Errorf("ExtractInstallSize = %d, want 50000", got)
	}
}

func TestExtractModuleFormat(t *testing.T) {
	files := []pypi.ReleaseFile{
		{Filename: "requests-2.31.0-py3-none-any.whl", PackageType: "bdist_wheel", Size: 62574},
	}

	got := ExtractModuleFormat(files)
	if got != "py3-none-any" {
		t.Errorf("ExtractModuleFormat = %q, want %q", got, "py3-none-any")
	}
}

func TestExtractModuleFormatPlatformSpecific(t *testing.T) {
	cases := []struct {
		filename string
	}{
		{"cryptography-41.0.0-cp39-cp39-manylinux_2_28_x86_64.whl"},
		{"Pillow-10.0.0-cp311-cp311-macosx_10_10_x86_64.whl"},
	}

	for _, tc := range cases {
		files := []pypi.ReleaseFile{
			{Filename: tc.filename, PackageType: "bdist_wheel", Size: 1000},
		}
		got := ExtractModuleFormat(files)
		if got != "platform-specific" {
			t.Errorf("ExtractModuleFormat(%q) = %q, want %q", tc.filename, got, "platform-specific")
		}
	}
}

func TestExtractPythonVersions(t *testing.T) {
	t.Run("simple >=3.8", func(t *testing.T) {
		info := ExtractPythonVersions(">=3.8")
		if info.MinVersion != "3.8" {
			t.Errorf("MinVersion = %q, want %q", info.MinVersion, "3.8")
		}
		if info.Constraint != ">=3.8" {
			t.Errorf("Constraint = %q, want %q", info.Constraint, ">=3.8")
		}
	})

	t.Run("compound >=3.9,<4.0", func(t *testing.T) {
		info := ExtractPythonVersions(">=3.9,<4.0")
		if info.MinVersion != "3.9" {
			t.Errorf("MinVersion = %q, want %q", info.MinVersion, "3.9")
		}
	})
}
