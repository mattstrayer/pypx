package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// githubURLPattern matches github.com/{owner}/{repo} with optional trailing
// path segments (e.g. /tree/main), .git suffix, or trailing slash.
var githubURLPattern = regexp.MustCompile(
	`(?i)github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)(?:[/#?]|\.git|$)`,
)

// priorityKeys defines the order in which project_urls keys are checked.
var priorityKeys = []string{
	"source",
	"repository",
	"source code",
	"github",
	"homepage",
	"code",
}

// ExtractGitHubRepo scans a PyPI project_urls map for a GitHub repository URL.
// Keys are matched case-insensitively in priority order; if none of the
// priority keys match, all values are scanned as a fallback.
// Returns (owner, repo, true) when found, or ("", "", false) otherwise.
func ExtractGitHubRepo(projectURLs map[string]string) (owner, repo string, ok bool) {
	if len(projectURLs) == 0 {
		return "", "", false
	}

	// Build a lowercase-key index for O(1) priority lookups.
	lower := make(map[string]string, len(projectURLs))
	for k, v := range projectURLs {
		lower[strings.ToLower(k)] = v
	}

	// Check priority keys first.
	for _, key := range priorityKeys {
		if v, exists := lower[key]; exists {
			if owner, repo, ok = extractFromURL(v); ok {
				return
			}
		}
	}

	// Fallback: scan all values.
	for _, v := range projectURLs {
		if owner, repo, ok = extractFromURL(v); ok {
			return
		}
	}

	return "", "", false
}

// extractFromURL attempts to parse a GitHub owner/repo from a single URL string.
func extractFromURL(rawURL string) (owner, repo string, ok bool) {
	m := githubURLPattern.FindStringSubmatch(rawURL)
	if m == nil {
		return "", "", false
	}
	owner = m[1]
	repo = strings.TrimSuffix(m[2], ".git")
	return owner, repo, true
}

// ghRelease is the internal JSON representation of a GitHub release.
type ghRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
}

// Release is the public representation of a GitHub release.
type Release struct {
	Version     string `json:"version"`
	TagName     string `json:"tag_name"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	BodyHTML    string `json:"body_html"`
	PublishedAt string `json:"published_at"`
	URL         string `json:"url"`
}

// Client fetches release data from the GitHub API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// Option is a functional option for configuring a Client.
type Option func(*Client)

// WithBaseURL overrides the default GitHub API base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithToken sets a GitHub personal access token for authenticated requests.
func WithToken(token string) Option {
	return func(c *Client) {
		c.token = token
	}
}

// NewClient creates a new GitHub API client with the given options.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL: "https://api.github.com",
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchReleases retrieves up to 100 releases for the given owner/repo.
// It returns an empty slice (not an error) on 404 or 403 responses.
func (c *Client) FetchReleases(owner, repo string) ([]Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100", c.baseURL, owner, repo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("github: failed to build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return []Release{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: unexpected status %d for %s/%s", resp.StatusCode, owner, repo)
	}

	var raw []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: failed to decode response: %w", err)
	}

	releases := make([]Release, len(raw))
	for i, r := range raw {
		releases[i] = Release{
			Version:     strings.TrimPrefix(r.TagName, "v"),
			TagName:     r.TagName,
			Title:       r.Name,
			Body:        r.Body,
			PublishedAt: r.PublishedAt,
			URL:         r.HTMLURL,
		}
	}
	return releases, nil
}
