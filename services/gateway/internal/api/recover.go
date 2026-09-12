package api

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/phuoctmse/settleguard/gateway/internal/httperror"
)

// RecoverJSON turns a handler panic into the gateway's standard JSON error
// body instead of chi's bare empty 500, so a client can keep one parser
// for every gateway-originated error (spec §8). The panic and stack go to
// the log; nothing about them reaches the caller.
func RecoverJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("gateway: panic request_id=%s: %v\n%s", r.Header.Get(HeaderRequestID), rec, debug.Stack())
				httperror.Write(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
