// Package api is the gateway's HTTP layer: the middleware chain, the
// router, and its configuration. It knows nothing about how keys are
// stored; that is internal/auth, reached through the KeyLookup interface.
package api

import (
	"encoding/json"
	"net/http"
)

// writeError sends the {"error": "..."} body every SettleGuard Go service
// uses, so a client needs one parser for gateway and upstream errors alike.
//
//nolint:unused // called by middleware throughout the api package
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Encode errors are not recoverable once headers are sent; client sees partial response either way.
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
