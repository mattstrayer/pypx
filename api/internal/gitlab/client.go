package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pypx/api/internal/circuitbreaker"
)

var gitlabURLPattern = regexp.MustCompile(
	`(?i)gitlab\.com/([A-Za-z0-9_.\-/]+?)(?:\.git|/?$)`,
)

var priorityKeys = []string{
	"source",
	"repository",
	"source code",
	"gitlab",
	"homepage",
	"code",
}

// ExtractGitLabRepo extracts the GitLab project path from PyPI project_urls.
// Returns (projectPath, repoURL, ok). projectPath is like "user/repo" or "group/sub/repo".
func ExtractGitLabRepo(projectURLs map[string]string) (projectPath, repoURL string, ok bool) {
	lower := make(map[string]string, len(projectURLs))
	for k, v := range projectURLs {
		lower[strings.ToLower(k)] = v
	}

	var candidates []string
	for _, key := range priorityKeys {
		if v, exists := lower[key]; exists {
			candidates = append(candidates, v)
		}
	}
	// Also scan all values as fallback.
	for _, v := range projectURLs {
		candidates = append(candidates, v)
	}

	for _, rawURL := range candidates {
		if m := gitlabURLPattern.FindStringSubmatch(rawURL); m != nil {
			path := strings.TrimSuffix(m[1], "/")
			// Require at least one slash (namespace/project).
			if !strings.Contains(path, "/") {
				continue
			}
			return path, "https://gitlab.com/" + path, true
		}
	}
	return "", "", false
}

// Release holds a normalized GitLab release.
type Release struct {
	Version     string
	TagName     string
	Title       string
	Body        string
	PublishedAt string
	URL         string
}

// Tag represents a git tag.
type Tag struct {
	Name string
	SHA  string
}

// Client is a minimal GitLab API client.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	breaker    *circuitbreaker.Breaker
}

type Option func(*Client)

func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}

func WithToken(tok string) Option {
	return func(c *Client) { c.token = tok }
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:    "https://gitlab.com",
		httpClient: &http.Client{Timeout: 15 * time.Second},
		breaker:    circuitbreaker.New(5, 30*time.Second),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) encodedPath(projectPath string) string {
	return url.PathEscape(projectPath)
}

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	if err := c.breaker.Allow(); err != nil {
		return nil, fmt.Errorf("gitlab: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.breaker.RecordFailure()
		return nil, err
	}
	if resp.StatusCode >= 500 {
		c.breaker.RecordFailure()
		return resp, nil
	}
	c.breaker.RecordSuccess()
	return resp, nil
}

var tagVersionRE = regexp.MustCompile(`^v?\d+\.\d[\d.a-zA-Z\-]*$`)

// FetchReleases fetches GitLab releases for the given project path.
func (c *Client) FetchReleases(ctx context.Context, projectPath string) ([]Release, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/api/v4/projects/%s/releases", c.encodedPath(projectPath)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab releases API returned %d", resp.StatusCode)
	}

	var raw []struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Description string `json:"description"`
		ReleasedAt  string `json:"released_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	releases := make([]Release, 0, len(raw))
	for _, r := range raw {
		version := strings.TrimPrefix(r.TagName, "v")
		releases = append(releases, Release{
			Version:     version,
			TagName:     r.TagName,
			Title:       r.Name,
			Body:        r.Description,
			PublishedAt: r.ReleasedAt,
			URL:         fmt.Sprintf("https://gitlab.com/%s/-/releases/%s", projectPath, r.TagName),
		})
	}
	return releases, nil
}

// FetchRawFile fetches the first file in candidates that exists in the repo.
// NOTE: Do NOT use defer inside the loop — bodies are closed explicitly before continue.
func (c *Client) FetchRawFile(ctx context.Context, projectPath string, candidates []string) (content, filename string, err error) {
	for _, name := range candidates {
		path := fmt.Sprintf("/api/v4/projects/%s/repository/files/%s/raw?ref=HEAD",
			c.encodedPath(projectPath), url.PathEscape(name))
		resp, err := c.get(ctx, path)
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusNotFound {
			_ = resp.Body.Close()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			continue
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			continue
		}
		return string(body), name, nil
	}
	return "", "", nil
}

// FetchTags returns version-like tags for the project, preserving API order (newest-first).
func (c *Client) FetchTags(ctx context.Context, projectPath string) ([]Tag, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/api/v4/projects/%s/repository/tags?per_page=50", c.encodedPath(projectPath)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var raw []struct {
		Name   string `json:"name"`
		Commit struct {
			ID string `json:"id"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	tags := make([]Tag, 0, len(raw))
	for _, t := range raw {
		if tagVersionRE.MatchString(t.Name) {
			tags = append(tags, Tag{Name: t.Name, SHA: t.Commit.ID})
		}
	}
	return tags, nil
}

var noiseCommitRE = regexp.MustCompile(`(?i)^(merge\b|bump version|version bump|chore[:(]|release[:(]|prepare release|update changelog)`)

// FetchCompare returns filtered commit messages and the date of the HEAD commit
// between base and head refs.
func (c *Client) FetchCompare(ctx context.Context, projectPath, base, head string) (messages []string, headDate string, err error) {
	path := fmt.Sprintf("/api/v4/projects/%s/repository/compare?from=%s&to=%s",
		c.encodedPath(projectPath), url.QueryEscape(base), url.QueryEscape(head))
	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("gitlab compare API returned %d", resp.StatusCode)
	}

	var payload struct {
		Commits []struct {
			Message      string `json:"message"`
			AuthoredDate string `json:"authored_date"`
		} `json:"commits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, "", err
	}

	const maxCommits = 30
	messages = make([]string, 0, len(payload.Commits))
	for _, cm := range payload.Commits {
		msg := strings.SplitN(cm.Message, "\n", 2)[0]
		if !noiseCommitRE.MatchString(msg) {
			messages = append(messages, msg)
			if len(messages) >= maxCommits {
				break
			}
		}
	}

	if n := len(payload.Commits); n > 0 {
		headDate = payload.Commits[n-1].AuthoredDate
	}
	return messages, headDate, nil
}
