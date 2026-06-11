package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/changelog"
	gh "github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/gitlab"
	"github.com/pypx/api/internal/markdown"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/api/internal/textfmt"
)

const changelogTTL = 7 * 24 * time.Hour

// ChangelogResponse is the JSON shape returned to the frontend.
type ChangelogResponse struct {
	Package string            `json:"package"`
	Source  string            `json:"source"`
	RepoURL string            `json:"repo_url"`
	Entries []changelog.Entry `json:"entries"`
}

// ChangelogHandler handles GET /api/packages/{name}/changelog.
type ChangelogHandler struct {
	github *gh.Client
	gitlab *gitlab.Client
	cache  cache.Cacher
	pkg    packageFetcher
}

// NewChangelogHandler creates a new ChangelogHandler.
func NewChangelogHandler(ghClient *gh.Client, glClient *gitlab.Client, c cache.Cacher, pkgHandler packageFetcher) *ChangelogHandler {
	return &ChangelogHandler{
		github: ghClient,
		gitlab: glClient,
		cache:  c,
		pkg:    pkgHandler,
	}
}

// fetchChangelog returns changelog data for a package, using the cache when available.
// Returns the decoded ChangelogResponse, the raw JSON bytes, whether the data was
// fresh (vs stale), and any error. On upstream failure, falls back to stale cache.
func (h *ChangelogHandler) fetchChangelog(ctx context.Context, name string) (ChangelogResponse, []byte, bool, error) {
	cacheKey := "changelog:" + strings.ToLower(name)

	// Serve from cache if available (fresh or stale).
	if data, fresh, err := h.cache.Get(cacheKey, changelogTTL); err == nil && data != nil {
		var resp ChangelogResponse
		if jsonErr := json.Unmarshal(data, &resp); jsonErr == nil {
			return resp, data, fresh, nil
		}
	}

	// Fetch PyPI package info to get project URLs.
	pypiResp, err := h.pkg.FetchPackage(ctx, name)
	if err != nil {
		if errors.Is(err, pypi.ErrNotFound) {
			return ChangelogResponse{}, nil, false, pypi.ErrNotFound
		}
		// Last resort: try serving any cached data regardless of TTL.
		if data, _, cacheErr := h.cache.Get(cacheKey, 0); cacheErr == nil && data != nil {
			var resp ChangelogResponse
			if jsonErr := json.Unmarshal(data, &resp); jsonErr == nil {
				return resp, data, false, nil
			}
		}
		return ChangelogResponse{}, nil, false, err
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp := h.buildResponse(fetchCtx, pypiResp.Info.Name, pypiResp.Info.ProjectURLs)

	encoded, err := json.Marshal(resp)
	if err != nil {
		return ChangelogResponse{}, nil, false, err
	}

	h.cache.Set(cacheKey, encoded, changelogTTL) //nolint:errcheck

	return resp, encoded, true, nil
}

// Get handles GET /api/packages/{name}/changelog.
func (h *ChangelogHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := pypi.ValidateName(name); err != nil {
		http.Error(w, "invalid package name", http.StatusBadRequest)
		return
	}

	_, encoded, fresh, err := h.fetchChangelog(r.Context(), name)
	if err != nil {
		if errors.Is(err, pypi.ErrNotFound) {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if fresh {
		w.Header().Set("Cache-Control", "public, max-age=604800")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=60")
	}
	w.Write(encoded) //nolint:errcheck
}

// GetText handles GET /api/packages/{name}/changelog.txt.
func (h *ChangelogHandler) GetText(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := pypi.ValidateName(name); err != nil {
		http.Error(w, "invalid package name", http.StatusBadRequest)
		return
	}

	resp, _, _, err := h.fetchChangelog(r.Context(), name)
	if err != nil {
		if errors.Is(err, pypi.ErrNotFound) {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return
	}

	body := textfmt.FormatChangelog(&textfmt.ChangelogInput{
		Package: resp.Package,
		Source:  resp.Source,
		RepoURL: resp.RepoURL,
		Entries: resp.Entries,
	})

	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body)) //nolint:errcheck
}

// buildResponse constructs a ChangelogResponse using the parallel registry.
func (h *ChangelogHandler) buildResponse(ctx context.Context, pkgName string, projectURLs map[string]string) ChangelogResponse {
	// Detect GitHub.
	if owner, repo, ok := gh.ExtractGitHubRepo(projectURLs); ok {
		repoURL := "https://github.com/" + owner + "/" + repo
		sources := []changelog.Source{
			gh.NewReleasesSource(h.github, owner, repo),
			gh.NewFileSource(h.github, owner, repo),
			gh.NewTagsSource(h.github, owner, repo),
		}
		reg := changelog.NewRegistry(sources...)
		result := reg.Fetch(ctx)

		// Render markdown bodies to HTML.
		entries := renderHTML(result.Entries)

		return ChangelogResponse{
			Package: pkgName,
			Source:  result.Source,
			RepoURL: repoURL,
			Entries: entries,
		}
	}

	// Detect GitLab.
	if projectPath, repoURL, ok := gitlab.ExtractGitLabRepo(projectURLs); ok {
		sources := []changelog.Source{
			gitlab.NewReleasesSource(h.gitlab, projectPath),
			gitlab.NewFileSource(h.gitlab, projectPath),
			gitlab.NewTagsSource(h.gitlab, projectPath),
		}
		reg := changelog.NewRegistry(sources...)
		result := reg.Fetch(ctx)
		entries := renderHTML(result.Entries)

		return ChangelogResponse{
			Package: pkgName,
			Source:  result.Source,
			RepoURL: repoURL,
			Entries: entries,
		}
	}

	// No recognizable repo URL.
	return ChangelogResponse{
		Package: pkgName,
		Source:  "none",
		Entries: []changelog.Entry{},
	}
}

// fetchChangelogSlice returns changelog entries for the package whose version
// falls in (from, to]. Versions are compared with compareVersions (defined in
// stubs.go). Returns (entries, "") on success or (nil, reason) on failure.
//
// Reasons:
//   - "no changelog source"   — no GitHub/GitLab repo or registry returned no entries
//   - "no entries in range"   — entries exist but none fall within (from, to]
//   - "fetch error"           — package metadata fetch or registry fetch errored
func (h *ChangelogHandler) fetchChangelogSlice(ctx context.Context, name, from, to string) ([]changelog.Entry, string) {
	resp, _, _, err := h.fetchChangelog(ctx, name)
	if err != nil {
		return nil, "fetch error"
	}
	if len(resp.Entries) == 0 || resp.Source == "none" {
		return nil, "no changelog source"
	}

	out := make([]changelog.Entry, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		if e.Version == "" {
			continue
		}
		// Include if from < e.Version <= to.
		if compareVersions(e.Version, from) <= 0 {
			continue
		}
		if compareVersions(e.Version, to) > 0 {
			continue
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, "no entries in range"
	}
	// Sort newest first by version. The upstream registry result is already
	// newest-first by published date in the typical case; this re-sort makes
	// the ordering explicit and resilient to source-specific quirks.
	sort.Slice(out, func(i, j int) bool {
		return compareVersions(out[i].Version, out[j].Version) > 0
	})
	return out, ""
}

// FetchChangelogSliceForTest is a test-only shim for fetchChangelogSlice.
// Production callers should use the unexported version.
func (h *ChangelogHandler) FetchChangelogSliceForTest(ctx context.Context, name, from, to string) ([]changelog.Entry, string) {
	return h.fetchChangelogSlice(ctx, name, from, to)
}

// renderHTML renders Body markdown to BodyHTML for each entry in-place.
func renderHTML(entries []changelog.Entry) []changelog.Entry {
	for i := range entries {
		if entries[i].Body != "" {
			html, err := markdown.RenderSafe(entries[i].Body)
			if err == nil {
				entries[i].BodyHTML = html
			}
		}
	}
	return entries
}
