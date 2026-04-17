package handler

import (
	"testing"

	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/goopy/model"
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

func TestNormalizeStubFiles(t *testing.T) {
	input := map[string][]byte{
		"django_stubs/__init__.pyi":           []byte("x: int"),
		"django_stubs/db/models/__init__.pyi": []byte("class QuerySet: ..."),
		"django_stubs/http/__init__.pyi":      []byte("class HttpRequest: ..."),
		"django_stubs-1.0.dist-info/METADATA": []byte("meta"),
	}
	topLevelPkgs := []string{"django_stubs"}

	files, pkgs := normalizeStubFiles(input, topLevelPkgs)

	wantFiles := map[string]bool{
		"django/__init__.py":           true,
		"django/db/models/__init__.py": true,
		"django/http/__init__.py":      true,
	}
	if len(files) != len(wantFiles) {
		t.Errorf("normalizeStubFiles: got %d files, want %d", len(files), len(wantFiles))
	}
	for path := range wantFiles {
		if _, ok := files[path]; !ok {
			t.Errorf("normalizeStubFiles: missing expected path %q", path)
		}
	}
	if len(pkgs) != 1 || pkgs[0] != "django" {
		t.Errorf("normalizeStubFiles: pkgs = %v, want [\"django\"]", pkgs)
	}
}

func TestNormalizeStubFiles_UnderscoreSuffix(t *testing.T) {
	input := map[string][]byte{
		"requests_stubs/__init__.pyi": []byte("x: int"),
	}
	_, pkgs := normalizeStubFiles(input, []string{"requests_stubs"})
	if len(pkgs) != 1 || pkgs[0] != "requests" {
		t.Errorf("normalizeStubFiles: pkgs = %v, want [\"requests\"]", pkgs)
	}
}

func TestNormalizeStubFiles_HyphenSuffix(t *testing.T) {
	input := map[string][]byte{
		"requests-stubs/__init__.pyi": []byte("x: int"),
	}
	_, pkgs := normalizeStubFiles(input, []string{"requests-stubs"})
	if len(pkgs) != 1 || pkgs[0] != "requests" {
		t.Errorf("normalizeStubFiles: pkgs = %v, want [\"requests\"]", pkgs)
	}
}

func TestBuildStubIndex(t *testing.T) {
	pkg := &model.Package{
		Modules: []*model.Module{
			{
				Name: "django.db.models",
				Functions: []*model.Function{
					{Name: "get_object_or_404"},
				},
				Classes: []*model.Class{
					{
						Name: "QuerySet",
						Methods: []*model.Function{
							{Name: "filter"},
							{Name: "exclude"},
						},
					},
				},
			},
		},
	}

	idx := buildStubIndex(pkg)

	if _, ok := idx["django.db.models.get_object_or_404"]; !ok {
		t.Error("buildStubIndex: missing module-level function key")
	}
	if _, ok := idx["django.db.models.QuerySet.filter"]; !ok {
		t.Error("buildStubIndex: missing class method key")
	}
	if _, ok := idx["django.db.models.QuerySet.exclude"]; !ok {
		t.Error("buildStubIndex: missing class method key")
	}
	if _, ok := idx["django.db.models.QuerySet"]; ok {
		t.Error("buildStubIndex: should not index class itself, only methods")
	}
}

func TestBuildStubIndex_NilPackage(t *testing.T) {
	idx := buildStubIndex(nil)
	if idx == nil {
		t.Error("buildStubIndex(nil) returned nil, want empty map")
	}
	if len(idx) != 0 {
		t.Errorf("buildStubIndex(nil) len = %d, want 0", len(idx))
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
		{"pyyaml", "types-PyYAML", true},
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
