package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/enrichment"
	"github.com/pypx/api/internal/markdown"
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
	Name           string                       `json:"name"`
	Version        string                       `json:"version"`
	Summary        string                       `json:"summary"`
	Description    string                       `json:"description"`
	DescType       string                       `json:"description_content_type"`
	DescriptionHTML string                      `json:"description_html"`
	License        string                       `json:"license"`
	Author         string                       `json:"author"`
	AuthorEmail    string                       `json:"author_email"`
	HomePage       string                       `json:"home_page"`
	RequiresPython string                       `json:"requires_python"`
	RequiresDist   []string                     `json:"requires_dist"`
	ProjectURLs    map[string]string            `json:"project_urls"`
	Classifiers    []string                     `json:"classifiers"`
	LatestFiles    []FileInfo                   `json:"latest_files"`
	InstallSize    int64                        `json:"install_size"`
	ModuleFormat   string                       `json:"module_format"`
	PythonVersions  enrichment.PythonVersionInfo `json:"python_versions"`
	Dependencies    enrichment.DependencyTree    `json:"dependencies"`
	PlatformCoverage enrichment.PlatformCoverage `json:"platform_coverage"`
	ReleaseCadence   enrichment.ReleaseCadence   `json:"release_cadence"`
	Maintainers      []enrichment.Maintainer     `json:"maintainers"`
	DocURL           string                       `json:"doc_url"`
}

// PackageHandler serves package metadata requests.
type PackageHandler struct {
	pypiClient *pypi.Client
	cache      cache.Cacher
}

// NewPackageHandler creates a new PackageHandler.
func NewPackageHandler(pypiClient *pypi.Client, c cache.Cacher) *PackageHandler {
	return &PackageHandler{pypiClient: pypiClient, cache: c}
}

// VersionInfo is a summary of a single release version.
type VersionInfo struct {
	Version      string     `json:"version"`
	InstallSize  int64      `json:"install_size"`
	ModuleFormat string     `json:"module_format"`
	UploadTime   string     `json:"upload_time"`
	Files        []FileInfo `json:"files"`
}

// fetchPackage retrieves a PyPIResponse, using the raw cache to avoid redundant
// upstream requests from both Get and GetVersions.
func (h *PackageHandler) fetchPackage(name string) (*pypi.PyPIResponse, error) {
	cacheKey := "raw:" + strings.ToLower(name)

	if data, _, err := h.cache.Get(cacheKey, packageTTL); err == nil && data != nil {
		var resp pypi.PyPIResponse
		if err := json.Unmarshal(data, &resp); err == nil {
			return &resp, nil
		}
	}

	resp, err := h.pypiClient.FetchPackage(name)
	if err != nil {
		return nil, err
	}

	if encoded, err := json.Marshal(resp); err == nil {
		h.cache.Set(cacheKey, encoded, packageTTL) //nolint:errcheck
	}

	return resp, nil
}

// FetchPackage is an exported wrapper around fetchPackage for use by other handlers.
func (h *PackageHandler) FetchPackage(name string) (*pypi.PyPIResponse, error) {
	return h.fetchPackage(name)
}

// fetchPackageForce fetches from PyPI and updates the raw cache, bypassing any
// cached value. Used for background revalidation.
func (h *PackageHandler) fetchPackageForce(name string) (*pypi.PyPIResponse, error) {
	resp, err := h.pypiClient.FetchPackage(name)
	if err != nil {
		return nil, err
	}

	cacheKey := "raw:" + strings.ToLower(name)
	if encoded, err := json.Marshal(resp); err == nil {
		h.cache.Set(cacheKey, encoded, packageTTL) //nolint:errcheck
	}

	return resp, nil
}

// validateName rejects invalid package names with a 400 response and returns
// false so the caller can return immediately.
func validateName(w http.ResponseWriter, name string) bool {
	if err := pypi.ValidateName(name); err != nil {
		http.Error(w, "invalid package name", http.StatusBadRequest)
		return false
	}
	return true
}

// Get handles GET /api/packages/{name}.
func (h *PackageHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}
	cacheKey := "pkg:" + strings.ToLower(name)

	// Check cache.
	if data, fresh, err := h.cache.Get(cacheKey, packageTTL); err == nil && data != nil {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("Content-Type", "application/json")
		w.Write(data) //nolint:errcheck

		if !fresh {
			// Serve stale data immediately and refresh in the background.
			go func() {
				resp, err := h.fetchPackageForce(name)
				if err != nil {
					log.Printf("background refresh failed for package %q: %v", name, err)
					return
				}
				pkg := buildPackageResponse(resp)
				encoded, err := json.Marshal(pkg)
				if err != nil {
					log.Printf("background refresh encode failed for package %q: %v", name, err)
					return
				}
				if err := h.cache.Set(cacheKey, encoded, packageTTL); err != nil {
					log.Printf("background refresh cache set failed for package %q: %v", name, err)
				}
			}()
		}
		return
	}

	// Cache miss — fetch from PyPI.
	resp, err := h.fetchPackage(name)
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

	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}

// GetDependencies handles GET /api/packages/{name}/dependencies.
func (h *PackageHandler) GetDependencies(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	pkg, err := h.fetchPackage(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return
	}

	tree := enrichment.ParseDependencies(pkg.Info.RequiresDist)

	encoded, err := json.Marshal(tree)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}

// GetVersions handles GET /api/packages/{name}/versions.
func (h *PackageHandler) GetVersions(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	resp, err := h.fetchPackage(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return
	}

	versions := make([]VersionInfo, 0, len(resp.Releases))
	for version, releaseFiles := range resp.Releases {
		if len(releaseFiles) == 0 {
			continue
		}

		files := make([]FileInfo, 0, len(releaseFiles))
		uploadTime := ""
		for _, f := range releaseFiles {
			files = append(files, FileInfo{
				Filename:    f.Filename,
				Size:        f.Size,
				PackageType: f.PackageType,
				PythonVer:   f.PythonVer,
				UploadTime:  f.UploadTime,
			})
			if uploadTime == "" {
				uploadTime = f.UploadTime
			}
		}

		versions = append(versions, VersionInfo{
			Version:      version,
			InstallSize:  enrichment.ExtractInstallSize(releaseFiles),
			ModuleFormat: enrichment.ExtractModuleFormat(releaseFiles),
			UploadTime:   uploadTime,
			Files:        files,
		})
	}

	// Sort by upload_time descending (newest first). Empty strings sort last.
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].UploadTime > versions[j].UploadTime
	})

	encoded, err := json.Marshal(versions)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded) //nolint:errcheck
}

// normalizeLicense extracts a short license name from PyPI metadata.
// It prefers license_expression (SPDX), then classifiers, then the raw
// license field (if short), and falls back to the first line of a long
// license if it looks like a known name.
func normalizeLicense(info pypi.PackageInfo) string {
	// 1. Prefer the modern SPDX license_expression field.
	if expr := strings.TrimSpace(info.LicenseExpression); expr != "" {
		return expr
	}

	// 2. Extract from classifiers (e.g. "License :: OSI Approved :: BSD License").
	for _, c := range info.Classifiers {
		if strings.HasPrefix(c, "License :: ") {
			parts := strings.Split(c, " :: ")
			if len(parts) >= 3 {
				// Use the most specific part (last segment).
				return parts[len(parts)-1]
			}
		}
	}

	// 3. If the raw license field is short, use it as-is.
	raw := strings.TrimSpace(info.License)
	if raw != "" && len(raw) <= 128 {
		return raw
	}

	// 4. For long license text, try the first non-empty line as a heuristic
	//    (many packages put "BSD 3-Clause License\n\nCopyright ...").
	if raw != "" {
		for _, line := range strings.SplitN(raw, "\n", 5) {
			line = strings.TrimSpace(line)
			if line != "" && len(line) <= 80 {
				return line
			}
			// Stop if we hit a long line — it's the license body.
			if len(line) > 80 {
				break
			}
		}
	}

	return ""
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

	var descHTML string
	if strings.Contains(info.DescriptionType, "text/markdown") {
		descHTML, _ = markdown.Render(info.Description)
	}

	return PackageResponse{
		Name:            info.Name,
		Version:         info.Version,
		Summary:         info.Summary,
		Description:     info.Description,
		DescType:        info.DescriptionType,
		DescriptionHTML: descHTML,
		License:         normalizeLicense(info),
		Author:         info.Author,
		AuthorEmail:    info.AuthorEmail,
		HomePage:       info.HomePage,
		RequiresPython: info.RequiresPython,
		RequiresDist:   info.RequiresDist,
		ProjectURLs:    info.ProjectURLs,
		Classifiers:    info.Classifiers,
		LatestFiles:    files,
		InstallSize:      enrichment.ExtractInstallSize(r.URLs),
		ModuleFormat:     enrichment.ExtractModuleFormat(r.URLs),
		PythonVersions:   enrichment.ExtractPythonVersions(info.RequiresPython),
		Dependencies:     enrichment.ParseDependencies(info.RequiresDist),
		PlatformCoverage: enrichment.ExtractPlatformCoverage(r.URLs),
		ReleaseCadence:   enrichment.ComputeReleaseCadence(r.Releases),
		Maintainers:      enrichment.ParseMaintainers(info),
		DocURL:           enrichment.ExtractDocURL(info.ProjectURLs),
	}
}
