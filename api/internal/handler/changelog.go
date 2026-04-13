package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/changelog"
	gh "github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/gitlab"
	"github.com/pypx/api/internal/markdown"
	"github.com/pypx/api/internal/pypi"
)

const changelogTTL = 7 * 24 * time.Hour

// ChangelogResponse is the JSON shape returned to the frontend.
type ChangelogResponse struct {
	Package  string            `json:"package"`
	Source   string            `json:"source"`
	RepoURL  string            `json:"repo_url"`
	Entries  []changelog.Entry `json:"entries"`
	RepoInfo *gh.RepoInfo      `json:"repo_info,omitempty"`
}

// ChangelogHandler handles GET /api/packages/{name}/changelog.
type ChangelogHandler struct {
	github *gh.Client
	gitlab *gitlab.Client
	cache  cache.Cacher
	pkg    *PackageHandler
}

// NewChangelogHandler creates a new ChangelogHandler.
func NewChangelogHandler(ghClient *gh.Client, glClient *gitlab.Client, c cache.Cacher, pkgHandler *PackageHandler) *ChangelogHandler {
	return &ChangelogHandler{
		github: ghClient,
		gitlab: glClient,
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

	// Serve from cache if fresh.
	if data, fresh, err := h.cache.Get(cacheKey, changelogTTL); err == nil && data != nil && fresh {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	// Fetch PyPI package info to get project URLs.
	pypiResp, err := h.pkg.FetchPackage(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp := h.buildResponse(ctx, pypiResp.Info.Name, pypiResp.Info.ProjectURLs)

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

// buildResponse constructs a ChangelogResponse using the parallel registry.
func (h *ChangelogHandler) buildResponse(ctx context.Context, pkgName string, projectURLs map[string]string) ChangelogResponse {
	// Detect GitHub.
	if owner, repo, ok := gh.ExtractGitHubRepo(projectURLs); ok {
		repoURL := "https://github.com/" + owner + "/" + repo
		sources := []changelog.Source{
			&gh.ReleasesSource{Client: h.github, Owner: owner, Repo: repo},
			&gh.FileSource{Client: h.github, Owner: owner, Repo: repo},
			&gh.TagsSource{Client: h.github, Owner: owner, Repo: repo},
		}
		reg := changelog.NewRegistry(sources...)
		result := reg.Fetch(ctx)

		// Render markdown bodies to HTML.
		entries := renderHTML(result.Entries)

		// Fetch repo metadata (stars, forks, etc.) for the sidebar.
		repoInfo, err := h.github.FetchRepoInfo(owner, repo)
		if err != nil {
			log.Printf("changelog: failed to fetch repo info for %s/%s: %v", owner, repo, err)
			repoInfo = nil
		}

		return ChangelogResponse{
			Package:  pkgName,
			Source:   result.Source,
			RepoURL:  repoURL,
			Entries:  entries,
			RepoInfo: repoInfo,
		}
	}

	// Detect GitLab.
	if projectPath, repoURL, ok := gitlab.ExtractGitLabRepo(projectURLs); ok {
		sources := []changelog.Source{
			&gitlab.ReleasesSource{Client: h.gitlab, ProjectPath: projectPath},
			&gitlab.FileSource{Client: h.gitlab, ProjectPath: projectPath},
			&gitlab.TagsSource{Client: h.gitlab, ProjectPath: projectPath},
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

// renderHTML renders Body markdown to BodyHTML for each entry in-place.
func renderHTML(entries []changelog.Entry) []changelog.Entry {
	for i := range entries {
		if entries[i].Body != "" {
			html, err := markdown.Render(entries[i].Body)
			if err == nil {
				entries[i].BodyHTML = html
			}
		}
	}
	return entries
}
