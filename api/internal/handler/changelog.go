package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	gh "github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/markdown"
	"github.com/pypx/api/internal/pypi"
)

const changelogTTL = 7 * 24 * time.Hour

// ChangelogResponse is the response returned by the changelog endpoint.
type ChangelogResponse struct {
	Package  string       `json:"package"`
	Source   string       `json:"source"`
	RepoURL  string       `json:"repo_url"`
	Entries  []gh.Release `json:"entries"`
	RepoInfo *gh.RepoInfo `json:"repo_info,omitempty"`
}

// ChangelogHandler serves changelog requests backed by GitHub Releases.
type ChangelogHandler struct {
	github *gh.Client
	cache  cache.Cacher
	pkg    *PackageHandler
}

// NewChangelogHandler creates a new ChangelogHandler.
func NewChangelogHandler(ghClient *gh.Client, c cache.Cacher, pkgHandler *PackageHandler) *ChangelogHandler {
	return &ChangelogHandler{
		github: ghClient,
		cache:  c,
		pkg:    pkgHandler,
	}
}

// Get handles GET /api/packages/{name}/changelog.
func (h *ChangelogHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := pypi.ValidateName(name); err != nil {
		http.Error(w, "invalid package name", http.StatusBadRequest)
		return
	}

	cacheKey := "changelog:" + strings.ToLower(name)

	// Check cache.
	if data, fresh, err := h.cache.Get(cacheKey, changelogTTL); err == nil && data != nil && fresh {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	// Fetch PyPI metadata to get project_urls.
	pypiResp, err := h.pkg.FetchPackage(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return
	}

	var releases []gh.Release
	repoURL := ""
	source := ""
	var repoInfo *gh.RepoInfo

	owner, repo, ok := gh.ExtractGitHubRepo(pypiResp.Info.ProjectURLs)
	if ok {
		repoURL = "https://github.com/" + owner + "/" + repo
		source = "github_releases"
		releases, err = h.github.FetchReleases(owner, repo)
		if err != nil {
			http.Error(w, "failed to fetch releases", http.StatusBadGateway)
			return
		}

		// Render markdown bodies to HTML.
		for i := range releases {
			if releases[i].Body != "" {
				releases[i].BodyHTML, _ = markdown.Render(releases[i].Body)
			}
		}

		repoInfo, err = h.github.FetchRepoInfo(owner, repo)
		if err != nil {
			log.Printf("changelog: failed to fetch repo info for %s/%s: %v", owner, repo, err)
			repoInfo = nil
		}
	}

	if releases == nil {
		releases = []gh.Release{}
	}

	resp := ChangelogResponse{
		Package:  pypiResp.Info.Name,
		Source:   source,
		RepoURL:  repoURL,
		Entries:  releases,
		RepoInfo: repoInfo,
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	h.cache.Set(cacheKey, encoded, changelogTTL) //nolint:errcheck

	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}
