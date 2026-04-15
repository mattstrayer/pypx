package wheel

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	DefaultMaxSize = 50 * 1024 * 1024 // 50 MB
	pypiBaseURL    = "https://pypi.org"
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
		HTTPClient: http.DefaultClient,
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
		return nil, fmt.Errorf("no wheel files found for %s==%s", name, version)
	}

	url := selectWheel(wheels)

	size, err := s.headSize(ctx, url)
	if err == nil && size > s.MaxSize {
		return nil, fmt.Errorf("wheel too large: %d bytes (max %d)", size, s.MaxSize)
	}

	data, err := s.download(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("downloading wheel: %w", err)
	}

	return extractPyFiles(data, name)
}

func (s *Source) fetchWheelURLs(ctx context.Context, name, version string) ([]WheelFile, error) {
	base := s.BaseURL
	if base == "" {
		base = pypiBaseURL
	}
	url := fmt.Sprintf("%s/pypi/%s/%s/json", base, name, version)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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

func selectWheel(wheels []WheelFile) string {
	for _, w := range wheels {
		if strings.Contains(w.Filename, "none-any") {
			return w.URL
		}
	}
	return wheels[0].URL
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
	resp.Body.Close()
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
	defer resp.Body.Close()
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

	for _, f := range r.File {
		if strings.Contains(f.Name, "__pycache__") {
			continue
		}

		if strings.HasSuffix(f.Name, ".dist-info/top_level.txt") {
			rc, err := f.Open()
			if err == nil {
				data, _ := io.ReadAll(rc)
				rc.Close()
				contents.TopLevelPkgs = parseTopLevelTxt(string(data))
			}
			continue
		}

		if strings.HasSuffix(f.Name, ".py") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			src, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
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
