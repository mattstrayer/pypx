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

// RepoOwner holds the GitHub account (user or org) that owns a repository.
type RepoOwner struct {
	Login       string `json:"login"`
	AvatarURL   string `json:"avatar_url"`
	DisplayName string `json:"display_name"` // org/user name, fetched separately
	URL         string `json:"url"`
	IsOrg       bool   `json:"is_org"`
}

// RepoInfo holds health signals for a GitHub repository.
type RepoInfo struct {
	Stars        int       `json:"stars"`
	Forks        int       `json:"forks"`
	OpenIssues   int       `json:"open_issues"`
	LastPushedAt string    `json:"last_pushed_at"`
	Owner        RepoOwner `json:"owner"`
}

// ghRepo is the internal JSON shape from GET /repos/{owner}/{repo}.
type ghRepo struct {
	StargazersCount int    `json:"stargazers_count"`
	ForksCount      int    `json:"forks_count"`
	OpenIssuesCount int    `json:"open_issues_count"`
	PushedAt        string `json:"pushed_at"`
	Owner           struct {
		Login     string `json:"login"`
		Type      string `json:"type"`
		AvatarURL string `json:"avatar_url"`
	} `json:"owner"`
}

// ghOrgUser is the internal JSON shape from GET /orgs/{login} or /users/{login}.
type ghOrgUser struct {
	Name string `json:"name"`
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

// FetchRepoInfo retrieves health signals for owner/repo.
// Returns nil (no error) on 404 or 403 — the package simply has no GitHub repo.
func (c *Client) FetchRepoInfo(owner, repo string) (*RepoInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: repo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: unexpected status %d for %s/%s", resp.StatusCode, owner, repo)
	}

	var raw ghRepo
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("github: decode repo: %w", err)
	}

	isOrg := strings.EqualFold(raw.Owner.Type, "Organization")
	displayName := c.fetchOwnerName(raw.Owner.Login, isOrg)

	ownerURL := "https://github.com/" + raw.Owner.Login
	return &RepoInfo{
		Stars:        raw.StargazersCount,
		Forks:        raw.ForksCount,
		OpenIssues:   raw.OpenIssuesCount,
		LastPushedAt: raw.PushedAt,
		Owner: RepoOwner{
			Login:       raw.Owner.Login,
			AvatarURL:   raw.Owner.AvatarURL,
			DisplayName: displayName,
			URL:         ownerURL,
			IsOrg:       isOrg,
		},
	}, nil
}

// fetchOwnerName calls /orgs/{login} or /users/{login} to get the display name.
// Returns login as fallback if the request fails.
func (c *Client) fetchOwnerName(login string, isOrg bool) string {
	path := "users"
	if isOrg {
		path = "orgs"
	}
	url := fmt.Sprintf("%s/%s/%s", c.baseURL, path, login)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return login
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return login
	}
	defer resp.Body.Close()

	var ou ghOrgUser
	if err := json.NewDecoder(resp.Body).Decode(&ou); err != nil || ou.Name == "" {
		return login
	}
	return ou.Name
}
