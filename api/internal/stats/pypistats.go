package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DataPoint represents a single download count entry.
type DataPoint struct {
	Category  string `json:"category"`
	Date      string `json:"date,omitempty"`
	Downloads int64  `json:"downloads"`
}

// StatsResponse is the top-level response from the pypistats.org API.
type StatsResponse struct {
	Data    []DataPoint `json:"data"`
	Package string      `json:"package"`
	Type    string      `json:"type"`
}

// Client fetches download statistics from pypistats.org.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Option is a functional option for configuring a Client.
type Option func(*Client)

// WithBaseURL overrides the default pypistats.org base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// NewClient creates a new pypistats client with the given options.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL: "https://pypistats.org",
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// fetch performs a GET request and decodes the JSON response into StatsResponse.
func (c *Client) fetch(ctx context.Context, url string) (*StatsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pypistats: build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pypistats: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("pypistats: resource not found: %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pypistats: unexpected status %d for %s", resp.StatusCode, url)
	}

	var result StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("pypistats: failed to decode response: %w", err)
	}

	return &result, nil
}

// FetchOverall retrieves overall download statistics for the named package.
func (c *Client) FetchOverall(ctx context.Context, name string) (*StatsResponse, error) {
	return c.fetch(ctx, fmt.Sprintf("%s/api/packages/%s/overall", c.baseURL, name))
}

// FetchPythonVersions retrieves per-Python-minor-version download stats for the named package.
func (c *Client) FetchPythonVersions(ctx context.Context, name string) (*StatsResponse, error) {
	return c.fetch(ctx, fmt.Sprintf("%s/api/packages/%s/python_minor", c.baseURL, name))
}

// FetchSystem retrieves per-OS download stats for the named package.
func (c *Client) FetchSystem(ctx context.Context, name string) (*StatsResponse, error) {
	return c.fetch(ctx, fmt.Sprintf("%s/api/packages/%s/system", c.baseURL, name))
}
