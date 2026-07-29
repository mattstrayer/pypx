package handler

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.gen.json
var openAPISpec []byte

// OpenAPI serves the generated OpenAPI 3.1 document describing the JSON API.
func OpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "application/json")
	w.Write(openAPISpec) //nolint:errcheck
}
