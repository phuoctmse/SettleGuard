// Package api is the gateway's HTTP layer: the middleware chain, the
// router, and its configuration. It knows nothing about how keys are
// stored; that is internal/auth, reached through the KeyLookup interface.
package api

import (
	"net/http"

	"github.com/google/uuid"
)

// HeaderRequestID is the header carrying the per-request trace id, both
// inbound (honoured if the client sent one) and outbound (always set).
const HeaderRequestID = "X-Request-Id"

// RequestID is the first middleware in the chain so that even a 401 or 429
// logged further down carries an id. It generates a UUID when the client
// did not send one, forwards it upstream on the request, and echoes it on
// the response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if id == "" {
			id = uuid.NewString()
			r.Header.Set(HeaderRequestID, id)
		}
		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r)
	})
}
