package wheel

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

const (
	DefaultMaxSize = 50 * 1024 * 1024 // 50 MB compressed download cap

	pypiBaseURL = "https://pypi.org"

	// maxDecompressedTotal caps the cumulative inflated size of all extracted
	// .py sources, guarding against zip bombs. Generous vs. real packages,
	// well under the 512 MB container.
	maxDecompressedTotal = 200 * 1024 * 1024 // 200 MB

	// maxDecompressedFile caps a single inflated .py entry.
	maxDecompressedFile = 32 * 1024 * 1024 // 32 MB

	// maxFileCount caps the number of extracted entries.
	maxFileCount = 50_000
)

// Sentinel errors. Callers use errors.Is to distinguish failure classes.
var (
	// ErrNotFound means PyPI has no such package/version (HTTP 404).
	ErrNotFound = errors.New("wheel: package version not found")
	// ErrNoArtifact means the release exists but has no extractable artifact
	// (e.g. sdist-only release, or no files at all).
	ErrNoArtifact = errors.New("wheel: no extractable artifact")
	// ErrTooLarge means the selected artifact exceeds MaxSize.
	ErrTooLarge = errors.New("wheel: artifact exceeds size limit")
)

// WheelFile represents a wheel distribution file from PyPI.
type WheelFile struct {
	Filename string
	URL      string
	Size     int64
}

// WheelContents holds the extracted .py files from a wheel.
type WheelContents struct {
	Files        map[string][]byte // relative path -> source bytes
	TopLevelPkgs []string
}

// Fetcher downloads and extracts Python source files from a package wheel.
type Fetcher interface {
	Fetch(ctx context.Context, name, version string) (*WheelContents, error)
}

// Source is the default Fetcher that downloads wheels from PyPI.
type Source struct {
	HTTPClient *http.Client
	MaxSize    int64
	BaseURL    string // PyPI base URL, defaults to https://pypi.org
}

// NewSource creates a new wheel Source with defaults.
func NewSource() *Source {
	return &Source{
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
		MaxSize:    DefaultMaxSize,
		BaseURL:    pypiBaseURL,
	}
}

// Compile-time check that Source implements Fetcher.
var _ Fetcher = (*Source)(nil)

// Fetch downloads a wheel for the given package and version, returning .py file contents.
func (s *Source) Fetch(ctx context.Context, name, version string) (*WheelContents, error) {
	wheels, err := s.fetchWheelURLs(ctx, name, version)
	if err != nil {
		return nil, fmt.Errorf("fetching wheel URLs: %w", err)
	}
	if len(wheels) == 0 {
		return nil, fmt.Errorf("%s==%s (sdist-only or no files): %w", name, version, ErrNoArtifact)
	}

	url := selectWheel(wheels)

	size, err := s.headSize(ctx, url)
	if err == nil && size > s.MaxSize {
		return nil, fmt.Errorf("%d bytes (max %d): %w", size, s.MaxSize, ErrTooLarge)
	}

	data, err := s.download(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("downloading wheel: %w", err)
	}
	if int64(len(data)) == s.MaxSize {
		return nil, fmt.Errorf("wheel too large: exceeds %d bytes (max %d)", s.MaxSize, s.MaxSize)
	}

	return extractPyFiles(data, name)
}

func (s *Source) fetchWheelURLs(ctx context.Context, name, version string) ([]WheelFile, error) {
	base := s.BaseURL
	if base == "" {
		base = pypiBaseURL
	}
	url := fmt.Sprintf("%s/pypi/%s/%s/json", base, neturl.PathEscape(name), neturl.PathEscape(version))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s==%s: %w", name, version, ErrNotFound)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("PyPI returned %d", resp.StatusCode)
	}

	var pypiResp struct {
		URLs []struct {
			Filename    string `json:"filename"`
			URL         string `json:"url"`
			Size        int64  `json:"size"`
			PackageType string `json:"packagetype"`
		} `json:"urls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pypiResp); err != nil {
		return nil, err
	}

	var wheels []WheelFile
	for _, u := range pypiResp.URLs {
		if u.PackageType == "bdist_wheel" {
			wheels = append(wheels, WheelFile{
				Filename: u.Filename,
				URL:      u.URL,
				Size:     u.Size,
			})
		}
	}
	return wheels, nil
}

// selectWheel picks one wheel deterministically: pure (none-any) wheels first,
// then smallest size, then lexicographically smallest filename. The result is
// independent of the order PyPI returns files in.
func selectWheel(wheels []WheelFile) string {
	best := wheels[0]
	for _, w := range wheels[1:] {
		if wheelLess(w, best) {
			best = w
		}
	}
	return best.URL
}

func wheelLess(a, b WheelFile) bool {
	ap, bp := strings.Contains(a.Filename, "none-any"), strings.Contains(b.Filename, "none-any")
	if ap != bp {
		return ap
	}
	if a.Size != b.Size {
		return a.Size < b.Size
	}
	return a.Filename < b.Filename
}

func (s *Source) headSize(ctx context.Context, url string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.ContentLength, nil
}

func (s *Source) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(io.LimitReader(resp.Body, s.MaxSize))
}

func extractPyFiles(data []byte, pkgName string) (*WheelContents, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}

	contents := &WheelContents{
		Files: make(map[string][]byte),
	}

	var totalDecompressed int64
	for _, f := range r.File {
		if strings.Contains(f.Name, "__pycache__") {
			continue
		}

		if len(contents.Files) >= maxFileCount {
			return nil, fmt.Errorf("wheel has too many files (max %d)", maxFileCount)
		}

		if strings.HasSuffix(f.Name, ".dist-info/top_level.txt") {
			rc, err := f.Open()
			if err == nil {
				data, _ := io.ReadAll(io.LimitReader(rc, maxDecompressedFile))
				_ = rc.Close()
				contents.TopLevelPkgs = parseTopLevelTxt(string(data))
			}
			continue
		}

		if strings.HasSuffix(f.Name, ".py") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			// Read one byte past the per-file cap to detect an over-limit entry.
			src, err := io.ReadAll(io.LimitReader(rc, maxDecompressedFile+1))
			_ = rc.Close()
			if err != nil {
				continue
			}
			if int64(len(src)) > maxDecompressedFile {
				return nil, fmt.Errorf("wheel file %q exceeds per-file decompression cap (%d bytes)", f.Name, maxDecompressedFile)
			}
			totalDecompressed += int64(len(src))
			if totalDecompressed > maxDecompressedTotal {
				return nil, fmt.Errorf("wheel exceeds total decompression budget (%d bytes)", maxDecompressedTotal)
			}
			contents.Files[f.Name] = src
		}
	}

	if len(contents.TopLevelPkgs) == 0 {
		contents.TopLevelPkgs = inferTopLevel(contents.Files, pkgName)
	}

	return contents, nil
}

func parseTopLevelTxt(content string) []string {
	var pkgs []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			pkgs = append(pkgs, line)
		}
	}
	return pkgs
}

func inferTopLevel(files map[string][]byte, pkgName string) []string {
	seen := map[string]bool{}
	for path := range files {
		parts := strings.SplitN(path, "/", 2)
		dir := parts[0]
		if len(parts) == 1 {
			// Bare top-level file like "six.py" — the module name is the stem.
			dir = strings.TrimSuffix(dir, ".py")
		}
		if strings.HasSuffix(dir, ".dist-info") || strings.HasSuffix(dir, ".data") {
			continue
		}
		seen[dir] = true
	}

	if len(seen) > 0 {
		var result []string
		for dir := range seen {
			result = append(result, dir)
		}
		return result
	}

	return []string{NormalizeName(pkgName)}
}

// NormalizeName converts a PyPI package name to a Python import name.
func NormalizeName(name string) string {
	result := strings.ToLower(name)
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, ".", "_")
	return result
}
