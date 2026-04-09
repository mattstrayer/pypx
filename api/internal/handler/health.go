package handler

import (
	"encoding/json"
	"net/http"
)

// Health returns a simple 200 OK with {"status":"ok"}.
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
}
