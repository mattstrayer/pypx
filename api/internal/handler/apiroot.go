package handler

import (
	"encoding/json"
	"net/http"
)

// APIRootResponse is the machine-readable index served at GET /api and
// GET /api/. It is also registered in the gentypes contract so the OpenAPI
// spec and generated TS type stay in sync with this handler's actual output.
type APIRootResponse struct {
	Description string `json:"description"`
	Llms        string `json:"llms"`
	Openapi     string `json:"openapi"`
	Source      string `json:"source"`
}

// APIRoot serves GET /api — a tiny machine-readable index so agents that
// probe the API root discover the agent surface instead of a 404.
func APIRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "application/json")
	body, err := json.Marshal(APIRootResponse{
		Description: "pypx API — agent-friendly PyPI frontend",
		Llms:        "/llms.txt",
		Openapi:     "/api/openapi.json",
		Source:      "https://github.com/mattstrayer/pypx",
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(append(body, '\n')) //nolint:errcheck
}
