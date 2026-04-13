package github

import (
	"encoding/json"
	"fmt"
	"io"
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

var tagVersionRE = regexp.MustCompile(`^v?\d+\.\d[\d.a-zA-Z\-]*$`)
var noiseCommitRE = regexp.MustCompile(`(?i)^(merge\b|bump version|version bump|chore[:(]|release[:(]|prepare release|update changelog)`)

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

// Tag represents a git tag with version-like name.
type Tag struct {
	Name string // original tag name (e.g. "v1.2.0")
	SHA  string // commit SHA
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

// FetchRawFile tries each candidate filename in order and returns the content and
// matched filename of the first one found. Returns ("", "", nil) if none exist.
func (c *Client) FetchRawFile(owner, repo string, candidates []string) (content, filename string, err error) {
	for _, name := range candidates {
		url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, owner, repo, name)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/vnd.github.raw+json")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}
		defer resp.Body.Close()
		var buf strings.Builder
		_, err = io.Copy(&buf, resp.Body)
		if err != nil {
			continue
		}
		return buf.String(), name, nil
	}
	return "", "", nil
}

// FetchTags retrieves up to 50 tags for owner/repo, filtering to version-like tags.
func (c *Client) FetchTags(owner, repo string) ([]Tag, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=50", c.baseURL, owner, repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var raw []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	tags := make([]Tag, 0, len(raw))
	for _, t := range raw {
		if tagVersionRE.MatchString(t.Name) {
			tags = append(tags, Tag{Name: t.Name, SHA: t.Commit.SHA})
		}
	}
	return tags, nil
}

// FetchCompare retrieves commit messages between base and head refs, filtering
// noise commits. Returns the messages and the date of the head commit.
func (c *Client) FetchCompare(owner, repo, base, head string) (messages []string, headDate string, err error) {
	url := fmt.Sprintf("%s/repos/%s/%s/compare/%s...%s", c.baseURL, owner, repo, base, head)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("compare API returned %d", resp.StatusCode)
	}

	var payload struct {
		Commits []struct {
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Date string `json:"date"`
				} `json:"author"`
			} `json:"commit"`
		} `json:"commits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, "", err
	}

	const maxCommits = 30
	messages = make([]string, 0, len(payload.Commits))
	for _, c := range payload.Commits {
		msg := strings.SplitN(c.Commit.Message, "\n", 2)[0]
		if !noiseCommitRE.MatchString(msg) {
			messages = append(messages, msg)
			if len(messages) >= maxCommits {
				break
			}
		}
	}

	if n := len(payload.Commits); n > 0 {
		headDate = payload.Commits[n-1].Commit.Author.Date
	}
	return messages, headDate, nil
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
	if err != nil {
		return login
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return login
	}

	var ou ghOrgUser
	if err := json.NewDecoder(resp.Body).Decode(&ou); err != nil || ou.Name == "" {
		return login
	}
	return ou.Name
}
