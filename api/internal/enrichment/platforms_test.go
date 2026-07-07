package enrichment_test

import (
	"testing"

	"github.com/pypx/api/internal/enrichment"
	"github.com/pypx/api/internal/pypi"
)

func TestExtractPlatformCoverage(t *testing.T) {
	tests := []struct {
		name  string
		files []pypi.ReleaseFile
		want  enrichment.PlatformCoverage
	}{
		{
			name: "pure python wheel",
			files: []pypi.ReleaseFile{
				{Filename: "requests-2.32.3-py3-none-any.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{PurePython: true},
		},
		{
			name: "linux x86 manylinux",
			files: []pypi.ReleaseFile{
				{Filename: "numpy-2.0.0-cp312-cp312-manylinux_2_17_x86_64.manylinux2014_x86_64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{LinuxX86: true},
		},
		{
			name: "macos arm64",
			files: []pypi.ReleaseFile{
				{Filename: "cryptography-42.0.0-cp312-cp312-macosx_14_0_arm64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{MacOSARM64: true},
		},
		{
			name: "windows amd64",
			files: []pypi.ReleaseFile{
				{Filename: "psutil-5.9.8-cp312-cp312-win_amd64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{Windows: true},
		},
		{
			name: "musl linux",
			files: []pypi.ReleaseFile{
				{Filename: "cryptography-42.0.0-cp312-cp312-musllinux_1_2_x86_64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{Musl: true},
		},
		{
			name: "linux arm64 aarch64",
			files: []pypi.ReleaseFile{
				{Filename: "numpy-2.0.0-cp312-cp312-manylinux_2_17_aarch64.manylinux2014_aarch64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{LinuxARM64: true},
		},
		{
			name: "macos x86 intel",
			files: []pypi.ReleaseFile{
				{Filename: "cryptography-42.0.0-cp312-cp312-macosx_10_9_x86_64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{MacOSX86: true},
		},
		{
			name: "universal2 sets both macos",
			files: []pypi.ReleaseFile{
				{Filename: "pillow-10.0.0-cp312-cp312-macosx_10_10_universal2.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{MacOSX86: true, MacOSARM64: true},
		},
		{
			name: "macos fat wheel dotted tag",
			files: []pypi.ReleaseFile{
				{Filename: "numpy-2.3.0-cp312-cp312-macosx_10_9_x86_64.macosx_11_0_arm64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{MacOSX86: true, MacOSARM64: true},
		},
		{
			name: "sdist only returns empty",
			files: []pypi.ReleaseFile{
				{Filename: "mypackage-1.0.0.tar.gz", PackageType: "sdist"},
			},
			want: enrichment.PlatformCoverage{},
		},
		{
			name: "multi-platform package",
			files: []pypi.ReleaseFile{
				{Filename: "cryptography-42.0.0-py3-none-any.whl", PackageType: "bdist_wheel"},
				{Filename: "cryptography-42.0.0-cp312-cp312-manylinux_2_17_x86_64.manylinux2014_x86_64.whl", PackageType: "bdist_wheel"},
				{Filename: "cryptography-42.0.0-cp312-cp312-win_amd64.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{PurePython: true, LinuxX86: true, Windows: true},
		},
		{
			name: "sdist alongside wheel - sdist ignored",
			files: []pypi.ReleaseFile{
				{Filename: "requests-2.32.3.tar.gz", PackageType: "sdist"},
				{Filename: "requests-2.32.3-py3-none-any.whl", PackageType: "bdist_wheel"},
			},
			want: enrichment.PlatformCoverage{PurePython: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enrichment.ExtractPlatformCoverage(tt.files)
			if got != tt.want {
				t.Errorf("ExtractPlatformCoverage() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
