package wheel

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"sort"
	"strings"
	"time"
)

const (
	DefaultMaxSize = 50 * 1024 * 1024 // 50 MB compressed download cap

	// DefaultMaxRangeSize caps the size of wheels read with range requests.
	// Only the central directory and .py entries are transferred.
	DefaultMaxRangeSize = 2 * 1024 * 1024 * 1024 // 2 GiB

	// rangeThreshold is the wheel size above which range requests replace a
	// whole download.
	rangeThreshold = 8 * 1024 * 1024 // 8 MiB

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
	// ErrTooLarge means the selected artifact exceeds MaxSize (or MaxRangeSize for ranged reads).
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
	// MaxRangeSize caps wheels read by range request; 0 means DefaultMaxRangeSize.
	MaxRangeSize int64
	BaseURL      string // PyPI base URL, defaults to https://pypi.org
}

// NewSource creates a new wheel Source with defaults.
func NewSource() *Source {
	return &Source{
		HTTPClient:   &http.Client{Timeout: 60 * time.Second},
		MaxSize:      DefaultMaxSize,
		MaxRangeSize: DefaultMaxRangeSize,
		BaseURL:      pypiBaseURL,
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

	return s.fetchWheel(ctx, selectWheel(wheels), name)
}

// fetchWheel reads one wheel. Wheels above rangeThreshold are read with HTTP
// range requests, so only the central directory and the .py entries cross the
// network. If the server refuses ranges, it falls back to a whole download.
func (s *Source) fetchWheel(ctx context.Context, w WheelFile, name string) (*WheelContents, error) {
	if w.Size <= rangeThreshold {
		return s.fetchWhole(ctx, w, name)
	}
	limit := s.MaxRangeSize
	if limit <= 0 {
		limit = DefaultMaxRangeSize
	}
	if w.Size > limit {
		return nil, fmt.Errorf("%d bytes (max %d): %w", w.Size, limit, ErrTooLarge)
	}
	ra, err := newHTTPReaderAt(ctx, s.HTTPClient, w.URL, w.Size)
	if err == nil {
		var contents *WheelContents
		contents, err = extractPyFiles(ra, w.Size, name)
		if err == nil {
			return contents, nil
		}
	}
	if errors.Is(err, errRangeUnsupported) {
		return s.fetchWhole(ctx, w, name)
	}
	return nil, fmt.Errorf("reading wheel by range: %w", err)
}

// fetchWhole downloads the whole wheel, which must fit within MaxSize.
func (s *Source) fetchWhole(ctx context.Context, w WheelFile, name string) (*WheelContents, error) {
	if w.Size > s.MaxSize {
		return nil, fmt.Errorf("%d bytes (max %d): %w", w.Size, s.MaxSize, ErrTooLarge)
	}
	data, err := s.download(ctx, w.URL)
	if err != nil {
		return nil, fmt.Errorf("downloading wheel: %w", err)
	}
	if int64(len(data)) > s.MaxSize {
		return nil, fmt.Errorf("download exceeds %d bytes: %w", s.MaxSize, ErrTooLarge)
	}
	return extractPyFiles(bytes.NewReader(data), int64(len(data)), name)
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
func selectWheel(wheels []WheelFile) WheelFile {
	best := wheels[0]
	for _, w := range wheels[1:] {
		if wheelLess(w, best) {
			best = w
		}
	}
	return best
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
	return io.ReadAll(io.LimitReader(resp.Body, s.MaxSize+1))
}

// isEntryCorrupt reports whether err means one zip entry holds bad data. Such
// an entry is skipped. Any other error (network failure, timeout, a server
// that stops honouring ranges) must abort extraction, or the caller would
// cache a partial result.
func isEntryCorrupt(err error) bool {
	var flateErr flate.CorruptInputError
	return errors.Is(err, zip.ErrFormat) || errors.Is(err, zip.ErrAlgorithm) ||
		errors.Is(err, zip.ErrChecksum) || errors.As(err, &flateErr)
}

func extractPyFiles(ra io.ReaderAt, size int64, pkgName string) (*WheelContents, error) {
	r, err := zip.NewReader(ra, size)
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}

	contents := &WheelContents{
		Files: make(map[string][]byte),
	}

	var totalDecompressed int64
	var declaredTopLevel []string
	for _, f := range r.File {
		if strings.Contains(f.Name, "__pycache__") {
			continue
		}

		if len(contents.Files) >= maxFileCount {
			return nil, fmt.Errorf("wheel has too many files (max %d)", maxFileCount)
		}

		if strings.HasSuffix(f.Name, ".dist-info/top_level.txt") {
			// Only the wheel's own metadata counts: exactly {name}.dist-info/top_level.txt.
			// Vendored distributions (pkg/_vendor/x.dist-info/...) must not overwrite it.
			if strings.Count(f.Name, "/") == 1 {
				rc, err := f.Open()
				if err != nil {
					if isEntryCorrupt(err) {
						continue
					}
					return nil, fmt.Errorf("opening %q: %w", f.Name, err)
				}
				data, err := io.ReadAll(io.LimitReader(rc, maxDecompressedFile))
				_ = rc.Close()
				if err != nil {
					if isEntryCorrupt(err) {
						continue
					}
					return nil, fmt.Errorf("reading %q: %w", f.Name, err)
				}
				declaredTopLevel = parseTopLevelTxt(string(data))
			}
			continue
		}

		if strings.HasSuffix(f.Name, ".py") {
			rc, err := f.Open()
			if err != nil {
				if isEntryCorrupt(err) {
					continue
				}
				return nil, fmt.Errorf("opening %q: %w", f.Name, err)
			}
			// Read one byte past the per-file cap to detect an over-limit entry.
			src, err := io.ReadAll(io.LimitReader(rc, maxDecompressedFile+1))
			_ = rc.Close()
			if err != nil {
				if isEntryCorrupt(err) {
					continue
				}
				return nil, fmt.Errorf("reading %q: %w", f.Name, err)
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

	// Keep only declared names that match a package directory or a module file.
	for _, name := range declaredTopLevel {
		if hasTopLevelFiles(contents.Files, name) {
			contents.TopLevelPkgs = append(contents.TopLevelPkgs, name)
		}
	}
	sort.Strings(contents.TopLevelPkgs)

	if len(contents.TopLevelPkgs) == 0 {
		contents.TopLevelPkgs = inferTopLevel(contents.Files, pkgName)
	}

	return contents, nil
}

func hasTopLevelFiles(files map[string][]byte, name string) bool {
	if _, ok := files[name+".py"]; ok {
		return true
	}
	prefix := name + "/"
	for path := range files {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
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
		sort.Strings(result)
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
