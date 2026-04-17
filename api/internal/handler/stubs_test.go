package handler

import (
	"testing"

	"github.com/pypx/api/internal/pypi"
)

func TestResolveStubVersion(t *testing.T) {
	releases := map[string][]pypi.ReleaseFile{
		"5.2.0": {},
		"5.1.3": {},
		"5.0.0": {},
		"4.2.1": {},
		"4.0.0": {},
	}

	tests := []struct {
		sourceMajor string
		want        string
	}{
		{"5", "5.2.0"}, // latest matching major
		{"4", "4.2.1"}, // latest matching major
		{"6", "5.2.0"}, // no match — fall back to overall latest
		{"3", "5.2.0"}, // no match — fall back to overall latest
	}
	for _, tt := range tests {
		got := resolveStubVersion(releases, tt.sourceMajor)
		if got != tt.want {
			t.Errorf("resolveStubVersion(releases, %q) = %q, want %q", tt.sourceMajor, got, tt.want)
		}
	}
}

func TestResolveStubVersion_EmptyReleases(t *testing.T) {
	got := resolveStubVersion(map[string][]pypi.ReleaseFile{}, "5")
	if got != "" {
		t.Errorf("resolveStubVersion(empty, \"5\") = %q, want \"\"", got)
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"5.2.0", "5.1.3", 1},
		{"5.1.3", "5.2.0", -1},
		{"5.2.0", "5.2.0", 0},
		{"10.0.0", "9.9.9", 1},
		{"1.0", "1.0.1", -1},
	}
	for _, tt := range tests {
		got := compareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestLookupStubPackage(t *testing.T) {
	tests := []struct {
		name     string
		wantStub string
		wantOK   bool
	}{
		{"django", "django-stubs", true},
		{"Django", "django-stubs", true},
		{"DJANGO", "django-stubs", true},
		{"djangorestframework", "djangorestframework-stubs", true},
		{"requests", "types-requests", true},
		{"boto3", "boto3-stubs", true},
		{"pandas", "pandas-stubs", true},
		{"pillow", "types-Pillow", true},
		{"redis", "types-redis", true},
		{"pyyaml", "types-PyYAML", true},
		{"stripe", "types-stripe", true},
		{"unknownpkg", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := lookupStubPackage(tt.name)
		if ok != tt.wantOK {
			t.Errorf("lookupStubPackage(%q) ok = %v, want %v", tt.name, ok, tt.wantOK)
		}
		if got != tt.wantStub {
			t.Errorf("lookupStubPackage(%q) = %q, want %q", tt.name, got, tt.wantStub)
		}
	}
}
