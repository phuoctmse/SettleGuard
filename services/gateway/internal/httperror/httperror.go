// Package httperror writes the one JSON error body every SettleGuard Go
// service uses, {"error": "..."}, so a client needs a single parser for
// gateway and upstream errors alike. It is a leaf package with no gateway
// imports, so both internal/api and internal/proxy can use it without an
// import cycle (api imports proxy).
package httperror

import (
	"encoding/json"
	"net/http"
)

// Write sends status with the body {"error": message}.
func Write(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Encode errors are not recoverable once headers are sent; client sees partial response either way.
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
