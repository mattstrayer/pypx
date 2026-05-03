package pypi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchPackage(t *testing.T) {
	expected := PyPIResponse{
		Info: PackageInfo{
			Name:           "requests",
			Version:        "2.31.0",
			Summary:        "Python HTTP for Humans.",
			License:        "Apache 2.0",
			Author:         "Kenneth Reitz",
			AuthorEmail:    "me@kennethreitz.org",
			RequiresPython: ">=3.7",
			RequiresDist:   []string{"charset-normalizer", "idna", "urllib3"},
			Classifiers:    []string{"Programming Language :: Python :: 3"},
			Keywords:       "http",
		},
		Releases: map[string][]ReleaseFile{
			"2.31.0": {
				{
					Filename:    "requests-2.31.0-py3-none-any.whl",
					URL:         "https://files.pythonhosted.org/packages/requests-2.31.0-py3-none-any.whl",
					Size:        62574,
					PackageType: "bdist_wheel",
					PythonVer:   "py3",
					UploadTime:  "2023-05-22T15:12:09.000000Z",
				},
			},
		},
		URLs: []ReleaseFile{
			{
				Filename:    "requests-2.31.0-py3-none-any.whl",
				URL:         "https://files.pythonhosted.org/packages/requests-2.31.0-py3-none-any.whl",
				Size:        62574,
				PackageType: "bdist_wheel",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pypi/requests/json" {
			t.Errorf("unexpected path: got %q, want /pypi/requests/json", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(expected); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	got, err := client.FetchPackage(context.Background(), "requests")
	if err != nil {
		t.Fatalf("FetchPackage returned unexpected error: %v", err)
	}

	if got.Info.Name != expected.Info.Name {
		t.Errorf("Info.Name: got %q, want %q", got.Info.Name, expected.Info.Name)
	}
	if got.Info.Version != expected.Info.Version {
		t.Errorf("Info.Version: got %q, want %q", got.Info.Version, expected.Info.Version)
	}
	if got.Info.Summary != expected.Info.Summary {
		t.Errorf("Info.Summary: got %q, want %q", got.Info.Summary, expected.Info.Summary)
	}
	if got.Info.License != expected.Info.License {
		t.Errorf("Info.License: got %q, want %q", got.Info.License, expected.Info.License)
	}
	if got.Info.RequiresPython != expected.Info.RequiresPython {
		t.Errorf("Info.RequiresPython: got %q, want %q", got.Info.RequiresPython, expected.Info.RequiresPython)
	}
	if len(got.Info.RequiresDist) != len(expected.Info.RequiresDist) {
		t.Errorf("Info.RequiresDist length: got %d, want %d", len(got.Info.RequiresDist), len(expected.Info.RequiresDist))
	}
	if len(got.Releases) != len(expected.Releases) {
		t.Errorf("Releases length: got %d, want %d", len(got.Releases), len(expected.Releases))
	}
	if files, ok := got.Releases["2.31.0"]; !ok {
		t.Error("Releases missing key 2.31.0")
	} else if len(files) != 1 {
		t.Errorf("Releases[2.31.0] length: got %d, want 1", len(files))
	} else {
		f := files[0]
		if f.Filename != expected.Releases["2.31.0"][0].Filename {
			t.Errorf("ReleaseFile.Filename: got %q, want %q", f.Filename, expected.Releases["2.31.0"][0].Filename)
		}
		if f.Size != expected.Releases["2.31.0"][0].Size {
			t.Errorf("ReleaseFile.Size: got %d, want %d", f.Size, expected.Releases["2.31.0"][0].Size)
		}
	}
	if len(got.URLs) != len(expected.URLs) {
		t.Errorf("URLs length: got %d, want %d", len(got.URLs), len(expected.URLs))
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{
		"requests",
		"flask-cors",
		"python_dateutil",
		"Jinja2",
		"A",         // single character
		"a1",        // alphanumeric
		"pkg.name",  // dot separator
	}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q): expected nil error, got %v", name, err)
		}
	}

	invalid := []string{
		"",
		"../etc/passwd",
		"foo/bar",
		"foo bar",
		".hidden",
		"-leading-hyphen",
		"trailing-hyphen-",
		"foo\\bar",
		"foo..bar",
	}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q): expected error, got nil", name)
		}
	}
}

func TestFetchPackageInvalidName(t *testing.T) {
	client := NewClient()
	got, err := client.FetchPackage(context.Background(), "../etc/passwd")
	if err == nil {
		t.Fatal("expected error for invalid package name, got nil")
	}
	if got != nil {
		t.Errorf("expected nil response on error, got %+v", got)
	}
}

func TestFetchPackageNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	got, err := client.FetchPackage(context.Background(), "nonexistent-package-xyz")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
	if got != nil {
		t.Errorf("expected nil response on error, got %+v", got)
	}
}
