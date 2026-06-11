package conda

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CondaForgeInfo describes a package's conda-forge availability.
type CondaForgeInfo struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	URL       string `json:"url,omitempty"`
}

// anacondaResponse is the partial JSON shape from the Anaconda API.
type anacondaResponse struct {
	Name          string `json:"name"`
	LatestVersion string `json:"latest_version"`
}

// Client queries the conda-forge channel via the Anaconda API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the Anaconda API base URL (for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// NewClient creates a new conda Client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:    "https://api.anaconda.org",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchCondaInfo checks whether name exists on conda-forge.
// Returns a CondaForgeInfo with Available=false (not an error) if the package
// is not found on conda-forge.
func (c *Client) FetchCondaInfo(ctx context.Context, name string) (CondaForgeInfo, error) {
	url := fmt.Sprintf("%s/package/conda-forge/%s", c.baseURL, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return CondaForgeInfo{}, fmt.Errorf("conda: build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return CondaForgeInfo{}, fmt.Errorf("conda: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return CondaForgeInfo{Available: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return CondaForgeInfo{}, fmt.Errorf("conda: unexpected status %d", resp.StatusCode)
	}

	var raw anacondaResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return CondaForgeInfo{}, fmt.Errorf("conda: decode response: %w", err)
	}

	return CondaForgeInfo{
		Available: true,
		Version:   raw.LatestVersion,
		URL:       fmt.Sprintf("https://anaconda.org/conda-forge/%s", name),
	}, nil
}
