package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pypx/api/internal/osv"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/api/internal/textfmt"
)

// SummaryHandler serves a composed agent briefing combining package metadata
// and (best-effort) security data.
type SummaryHandler struct {
	pkg packageFetcher
	osv *osv.Client
}

// NewSummaryHandler creates a new SummaryHandler.
func NewSummaryHandler(pkg packageFetcher, osvClient *osv.Client) *SummaryHandler {
	return &SummaryHandler{pkg: pkg, osv: osvClient}
}

// Get handles GET /api/packages/{name}/summary.txt.
func (h *SummaryHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !validateName(w, name) {
		return
	}

	resp, err := h.pkg.FetchPackage(r.Context(), name)
	if err != nil {
		if errors.Is(err, pypi.ErrNotFound) {
			http.Error(w, "package not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch package", http.StatusBadGateway)
		return
	}

	pkg := buildPackageResponse(resp)
	pkgInput := packageInputFrom(pkg)

	// Best-effort security: ignore errors, omit section if it fails.
	var secInput *textfmt.SecurityInput
	if vulns, err := h.osv.FetchVulns(r.Context(), name, ""); err == nil {
		secInput = &textfmt.SecurityInput{Package: name, Vulns: vulns}
	}

	body := textfmt.FormatSummary(&textfmt.SummaryInput{
		Package:  &pkgInput,
		Security: secInput,
	})

	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body)) //nolint:errcheck
}
