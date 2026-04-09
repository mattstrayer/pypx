package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/pypi"
)

const packageTTL = time.Hour

// FileInfo is a flattened representation of a single release file.
type FileInfo struct {
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	PackageType string `json:"package_type"`
	PythonVer   string `json:"python_version"`
	UploadTime  string `json:"upload_time"`
}

// PackageResponse is the flattened package metadata returned by the API.
type PackageResponse struct {
	Name           string            `json:"name"`
	Version        string            `json:"version"`
	Summary        string            `json:"summary"`
	Description    string            `json:"description"`
	DescType       string            `json:"description_content_type"`
	License        string            `json:"license"`
	Author         string            `json:"author"`
	AuthorEmail    string            `json:"author_email"`
	HomePage       string            `json:"home_page"`
	RequiresPython string            `json:"requires_python"`
	RequiresDist   []string          `json:"requires_dist"`
	ProjectURLs    map[string]string `json:"project_urls"`
	Classifiers    []string          `json:"classifiers"`
	LatestFiles    []FileInfo        `json:"latest_files"`
}

// PackageHandler serves package metadata requests.
type PackageHandler struct {
	pypiClient *pypi.Client
	cache      *cache.Cache
}

// NewPackageHandler creates a new PackageHandler.
func NewPackageHandler(pypiClient *pypi.Client, c *cache.Cache) *PackageHandler {
	return &PackageHandler{pypiClient: pypiClient, cache: c}
}

// Get handles GET /api/packages/{name}.
func (h *PackageHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	cacheKey := "pkg:" + strings.ToLower(name)

	// Check cache.
	if data, fresh, err := h.cache.Get(cacheKey, packageTTL); err == nil && data != nil {
		if fresh {
			w.Header().Set("Content-Type", "application/json")
			w.Write(data) //nolint:errcheck
			return
		}
		// Stale: serve stale data; background refresh is handled elsewhere.
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck
		return
	}

	// Cache miss — fetch from PyPI.
	resp, err := h.pypiClient.FetchPackage(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return
	}

	pkg := buildPackageResponse(resp)

	encoded, err := json.Marshal(pkg)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	// Cache for future requests.
	h.cache.Set(cacheKey, encoded, packageTTL) //nolint:errcheck

	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}

func buildPackageResponse(r *pypi.PyPIResponse) PackageResponse {
	info := r.Info

	files := make([]FileInfo, 0, len(r.URLs))
	for _, f := range r.URLs {
		files = append(files, FileInfo{
			Filename:    f.Filename,
			Size:        f.Size,
			PackageType: f.PackageType,
			PythonVer:   f.PythonVer,
			UploadTime:  f.UploadTime,
		})
	}

	return PackageResponse{
		Name:           info.Name,
		Version:        info.Version,
		Summary:        info.Summary,
		Description:    info.Description,
		DescType:       info.DescriptionType,
		License:        info.License,
		Author:         info.Author,
		AuthorEmail:    info.AuthorEmail,
		HomePage:       info.HomePage,
		RequiresPython: info.RequiresPython,
		RequiresDist:   info.RequiresDist,
		ProjectURLs:    info.ProjectURLs,
		Classifiers:    info.Classifiers,
		LatestFiles:    files,
	}
}
