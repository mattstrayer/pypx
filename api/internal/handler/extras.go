package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/conda"
	gh "github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/api/internal/textfmt"
)

const extrasTTL = 24 * time.Hour

// ExtrasResponse is the response for GET /api/packages/{name}/extras.
type ExtrasResponse struct {
	Package     string                `json:"package"`
	TypeSupport pypi.TypeSupport      `json:"type_support"`
	CondaForge  *conda.CondaForgeInfo `json:"conda_forge"`
	RepoInfo    *gh.RepoInfo          `json:"repo_info,omitempty"`
}

// ExtrasHandler serves type support, conda-forge, and GitHub repo data.
type ExtrasHandler struct {
	pypi   *pypi.Client
	conda  *conda.Client
	github *gh.Client
	pkg    packageFetcher
	cache  cache.Cacher
}

// NewExtrasHandler creates a new ExtrasHandler.
func NewExtrasHandler(pypiClient *pypi.Client, condaClient *conda.Client, ghClient *gh.Client, pkgHandler packageFetcher, c cache.Cacher) *ExtrasHandler {
	return &ExtrasHandler{pypi: pypiClient, conda: condaClient, github: ghClient, pkg: pkgHandler, cache: c}
}

// fetchExtras fetches extras data for a package, using the cache when available.
// Returns the JSON-encoded response and the decoded ExtrasResponse.
func (h *ExtrasHandler) fetchExtras(ctx context.Context, name string) (*ExtrasResponse, []byte, error) {
	cacheKey := "extras:" + strings.ToLower(name)

	if data, _, err := h.cache.Get(cacheKey, extrasTTL); err == nil && data != nil {
		var resp ExtrasResponse
		if jsonErr := json.Unmarshal(data, &resp); jsonErr == nil {
			return &resp, data, nil
		}
	}

	// Fetch PyPI package info (cached, fast) to get project URLs for repo detection.
	var pypiResp *pypi.PyPIResponse
	if h.pkg != nil {
		pypiResp, _ = h.pkg.FetchPackage(ctx, name)
	}

	// Fetch type support, conda info, and GitHub repo info in parallel.
	var (
		typeSupport pypi.TypeSupport
		condaInfo   conda.CondaForgeInfo
		condaErr    error
		repoInfo    *gh.RepoInfo
		wg          sync.WaitGroup
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		typeSupport = pypi.CheckTypeSupport(ctx, h.pypi, name)
	}()
	go func() {
		defer wg.Done()
		condaInfo, condaErr = h.conda.FetchCondaInfo(ctx, name)
	}()
	go func() {
		defer wg.Done()
		if pypiResp == nil || h.github == nil {
			return
		}
		owner, repo, ok := gh.ExtractGitHubRepo(pypiResp.Info.ProjectURLs)
		if !ok {
			return
		}
		info, err := h.github.FetchRepoInfo(ctx, owner, repo)
		if err == nil {
			repoInfo = info
		}
	}()
	wg.Wait()

	// If not already typed via stubs, check for py.typed marker in the wheel.
	if typeSupport.Status != "typed" && pypiResp != nil {
		typedKey := "typed:" + strings.ToLower(name) + ":" + pypiResp.Info.Version
		if data, _, err := h.cache.Get(typedKey, 0); err == nil && data != nil {
			if string(data) == "1" {
				typeSupport.Status = "typed"
			}
		} else {
			wheelURL := pypi.ExtractWheelURL(pypiResp.URLs)
			if wheelURL != "" && pypi.CheckPyTyped(ctx, h.pypi, wheelURL) {
				typeSupport.Status = "typed"
				h.cache.Set(typedKey, []byte("1"), 0) //nolint:errcheck
			} else {
				// Cache negative — either no wheel or wheel has no py.typed marker. Immutable per version.
				h.cache.Set(typedKey, []byte("0"), 0) //nolint:errcheck
			}
		}
	}

	resp := ExtrasResponse{
		Package:     name,
		TypeSupport: typeSupport,
		RepoInfo:    repoInfo,
	}
	if condaErr == nil {
		resp.CondaForge = &condaInfo
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		return nil, nil, err
	}

	h.cache.Set("extras:"+strings.ToLower(name), encoded, extrasTTL) //nolint:errcheck

	return &resp, encoded, nil
}

// Get handles GET /api/packages/{name}/extras.
func (h *ExtrasHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	_, encoded, err := h.fetchExtras(r.Context(), name)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}

// GetText handles GET /api/packages/{name}/extras.txt.
func (h *ExtrasHandler) GetText(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	resp, _, err := h.fetchExtras(r.Context(), name)
	if err != nil {
		http.Error(w, "failed to fetch extras data", http.StatusInternalServerError)
		return
	}

	body := textfmt.FormatExtras(extrasResponseToInput(resp))

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body)) //nolint:errcheck
}

// extrasResponseToInput converts an ExtrasResponse to a textfmt.ExtrasInput.
func extrasResponseToInput(resp *ExtrasResponse) *textfmt.ExtrasInput {
	in := &textfmt.ExtrasInput{
		Package:    resp.Package,
		TypeStatus: resp.TypeSupport.Status,
	}
	if resp.TypeSupport.Status == "stubs" {
		in.StubPackage = resp.TypeSupport.StubsPackage
	}
	if resp.CondaForge != nil && resp.CondaForge.Available {
		in.CondaAvailable = true
		in.CondaLatest = resp.CondaForge.Version
	}
	if resp.RepoInfo != nil {
		in.HasRepo = true
		in.RepoStars = resp.RepoInfo.Stars
		in.RepoForks = resp.RepoInfo.Forks
		in.RepoOpenIssues = resp.RepoInfo.OpenIssues
	}
	return in
}
