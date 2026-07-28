package handler

import "net/http"

// APIRoot serves GET /api — a tiny machine-readable index so agents that
// probe the API root discover the agent surface instead of a 404.
func APIRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"description":"pypx API — agent-friendly PyPI frontend","llms":"/llms.txt","openapi":"/api/openapi.json","source":"https://github.com/mattstrayer/pypx"}` + "\n")) //nolint:errcheck
}
