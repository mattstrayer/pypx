package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/goopy"
	"github.com/pypx/goopy/model"
)

// stubHTTPClient is used for downloading stub wheels with an explicit timeout.
var stubHTTPClient = &http.Client{Timeout: 30 * time.Second}

// stubIndex maps qualified symbol names to stub functions.
// Keys: "module.FuncName" for functions, "module.ClassName.methodName" for methods.
type stubIndex map[string]*model.Function

// stubRegistry maps lowercase PyPI package names to their known type stub packages.
// Only add entries for verified, well-maintained stub packages.
var stubRegistry = map[string]string{
	"django":              "django-stubs",
	"djangorestframework": "djangorestframework-stubs",
	"requests":            "types-requests",
	"boto3":               "boto3-stubs",
	"pandas":              "pandas-stubs",
	"pyyaml":              "types-PyYAML",
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

// normalizeStubFiles renames .pyi files to .py and maps stub top-level package
// directories to their source equivalents (e.g. django_stubs/ → django/).
// Non-.pyi files (dist-info, etc.) are excluded.
func normalizeStubFiles(files map[string][]byte, topLevelPkgs []string) (map[string][]byte, []string) {
	renames := make(map[string]string, len(topLevelPkgs))
	normalized := make([]string, 0, len(topLevelPkgs))
	for _, pkg := range topLevelPkgs {
		src := strings.TrimSuffix(strings.TrimSuffix(pkg, "_stubs"), "-stubs")
		renames[pkg] = src
		normalized = append(normalized, src)
	}

	result := make(map[string][]byte, len(files))
	for path, data := range files {
		if !strings.HasSuffix(path, ".pyi") {
			continue
		}
		// .pyi → .py
		p := strings.TrimSuffix(path, ".pyi") + ".py"
		// stub dir → source dir
		for stubDir, srcDir := range renames {
			if strings.HasPrefix(p, stubDir+"/") {
				p = srcDir + "/" + p[len(stubDir)+1:]
				break
			}
		}
		result[p] = data
	}
	return result, normalized
}

// fetchStubPackage downloads a stub wheel, extracts and normalizes .pyi files,
// and returns a parsed model.Package. Returns nil, nil if no wheel URL is found.
func fetchStubPackage(ctx context.Context, stubPkgName string, releases map[string][]pypi.ReleaseFile, version string) (*model.Package, error) {
	files := releases[version]
	var wheelURL string
	for _, f := range files {
		if strings.HasSuffix(f.Filename, ".whl") {
			wheelURL = f.URL
			break
		}
	}
	if wheelURL == "" {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wheelURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := stubHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchStubPackage: HTTP %d fetching %s", resp.StatusCode, wheelURL)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	rawFiles := make(map[string][]byte)
	topLevelSet := make(map[string]bool)
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".pyi") {
			continue
		}
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		topLevelSet[parts[0]] = true
		rc, err := f.Open()
		if err != nil {
			continue
		}
		src, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			continue
		}
		rawFiles[f.Name] = src
	}

	topLevelPkgs := make([]string, 0, len(topLevelSet))
	for pkg := range topLevelSet {
		topLevelPkgs = append(topLevelPkgs, pkg)
	}

	normalizedFiles, normalizedPkgs := normalizeStubFiles(rawFiles, topLevelPkgs)
	pkg := goopy.ExtractPackage(ctx, stubPkgName, normalizedFiles, normalizedPkgs)
	return pkg, nil
}

// buildStubIndex builds a flat lookup map from a parsed stub package.
func buildStubIndex(pkg *model.Package) stubIndex {
	idx := make(stubIndex)
	if pkg == nil {
		return idx
	}
	for _, mod := range pkg.Modules {
		for _, fn := range mod.Functions {
			idx[mod.Name+"."+fn.Name] = fn
		}
		for _, cls := range mod.Classes {
			for _, m := range cls.Methods {
				idx[mod.Name+"."+cls.Name+"."+m.Name] = m
			}
		}
	}
	return idx
}
