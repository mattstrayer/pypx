package pypi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PackageInfo holds the metadata from the PyPI "info" field.
type PackageInfo struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Summary         string            `json:"summary"`
	Description     string            `json:"description"`
	DescriptionType string            `json:"description_content_type"`
	License         string            `json:"license"`
	Author          string            `json:"author"`
	AuthorEmail     string            `json:"author_email"`
	HomePage        string            `json:"home_page"`
	RequiresPython  string            `json:"requires_python"`
	RequiresDist    []string          `json:"requires_dist"`
	ProjectURLs     map[string]string `json:"project_urls"`
	Classifiers     []string          `json:"classifiers"`
	Keywords        string            `json:"keywords"`
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
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchPackage retrieves the PyPI JSON API response for the named package.
func (c *Client) FetchPackage(name string) (*PyPIResponse, error) {
	url := fmt.Sprintf("%s/pypi/%s/json", c.baseURL, name)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("pypi: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("pypi: package %q not found", name)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pypi: unexpected status %d for package %q", resp.StatusCode, name)
	}

	var result PyPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("pypi: failed to decode response: %w", err)
	}

	return &result, nil
}
