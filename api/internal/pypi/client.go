package pypi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/pypx/api/internal/circuitbreaker"
)

// ErrNotFound is returned by FetchPackage when the package does not exist on PyPI.
var ErrNotFound = errors.New("pypi: package not found")

// validPackageName matches valid PyPI package names: alphanumeric, hyphens,
// underscores, and dots, starting and ending with an alphanumeric character.
var validPackageName = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)

// ValidateName returns an error if name is not a valid PyPI package name.
// It rejects empty strings, consecutive dots (path traversal), and any
// characters outside the allowed set (alphanumeric, hyphen, underscore, dot).
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("pypi: package name must not be empty")
	}
	// Block consecutive dots before regex to catch "foo..bar" style traversal.
	if strings.Contains(name, "..") {
		return fmt.Errorf("pypi: invalid package name %q", name)
	}
	if !validPackageName.MatchString(name) {
		return fmt.Errorf("pypi: invalid package name %q", name)
	}
	return nil
}

// PackageInfo holds the metadata from the PyPI "info" field.
type PackageInfo struct {
	Name              string            `json:"name"`
	Version           string            `json:"version"`
	Summary           string            `json:"summary"`
	Description       string            `json:"description"`
	DescriptionType   string            `json:"description_content_type"`
	License           string            `json:"license"`
	LicenseExpression string            `json:"license_expression"`
	Author            string            `json:"author"`
	AuthorEmail       string            `json:"author_email"`
	Maintainer        string            `json:"maintainer"`
	MaintainerEmail   string            `json:"maintainer_email"`
	HomePage          string            `json:"home_page"`
	RequiresPython    string            `json:"requires_python"`
	RequiresDist      []string          `json:"requires_dist"`
	ProjectURLs       map[string]string `json:"project_urls"`
	Classifiers       []string          `json:"classifiers"`
	Keywords          string            `json:"keywords"`
}

// ReleaseFile represents a single distribution file for a release.
type ReleaseFile struct {
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	Size        int64  `json:"size"`
	PackageType string `json:"packagetype"`
	PythonVer   string `json:"python_version"`
	RequiresPy  string `json:"requires_python"`
	UploadTime  string `json:"upload_time_iso_8601"`
	Digests     struct {
		SHA256 string `json:"sha256"`
	} `json:"digests"`
}

// PyPIResponse is the top-level response from the PyPI JSON API.
type PyPIResponse struct {
	Info     PackageInfo               `json:"info"`
	Releases map[string][]ReleaseFile  `json:"releases"`
	URLs     []ReleaseFile             `json:"urls"`
}

// Client fetches package metadata from the PyPI JSON API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	breaker    *circuitbreaker.Breaker
}

// Option is a functional option for configuring a Client.
type Option func(*Client)

// WithBaseURL overrides the default PyPI base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// NewClient creates a new PyPI client with the given options.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL: "https://pypi.org",
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		breaker: circuitbreaker.New(5, 30*time.Second),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchPackage retrieves the PyPI JSON API response for the named package.
func (c *Client) FetchPackage(ctx context.Context, name string) (*PyPIResponse, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	if err := c.breaker.Allow(); err != nil {
		return nil, fmt.Errorf("pypi: %w", err)
	}

	url := fmt.Sprintf("%s/pypi/%s/json", c.baseURL, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pypi: build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.breaker.RecordFailure()
		return nil, fmt.Errorf("pypi: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		// 404 is a valid "not found" response — not a service failure.
		return nil, fmt.Errorf("pypi: package %q: %w", name, ErrNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		c.breaker.RecordFailure()
		return nil, fmt.Errorf("pypi: unexpected status %d for package %q", resp.StatusCode, name)
	}

	var result PyPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.breaker.RecordFailure()
		return nil, fmt.Errorf("pypi: failed to decode response: %w", err)
	}

	c.breaker.RecordSuccess()
	return &result, nil
}

// FetchPackageAtVersion retrieves the PyPI JSON for a specific version. The
// response shape is identical to FetchPackage, but Info.RequiresDist and other
// metadata are pinned to that version (rather than the latest). Used by the
// diff endpoint to compare requires_dist between releases.
func (c *Client) FetchPackageAtVersion(ctx context.Context, name, version string) (*PyPIResponse, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	if version == "" {
		return nil, fmt.Errorf("pypi: empty version")
	}

	if err := c.breaker.Allow(); err != nil {
		return nil, fmt.Errorf("pypi: %w", err)
	}

	url := fmt.Sprintf("%s/pypi/%s/%s/json", c.baseURL, name, version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pypi: build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.breaker.RecordFailure()
		return nil, fmt.Errorf("pypi: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("pypi: %q@%s: %w", name, version, ErrNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		c.breaker.RecordFailure()
		return nil, fmt.Errorf("pypi: unexpected status %d for %q@%s", resp.StatusCode, name, version)
	}

	var result PyPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.breaker.RecordFailure()
		return nil, fmt.Errorf("pypi: failed to decode response: %w", err)
	}

	c.breaker.RecordSuccess()
	return &result, nil
}
