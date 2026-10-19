package api

import (
	"net/http"

	"github.com/phuoctmse/settleguard/gateway/internal/httperror"
)

// MaxRequestBodyBytes caps every request body the gateway forwards. The
// largest legitimate body is a ledger transaction of a few entries, well
// under a kilobyte; 1 MiB leaves room without letting one request fill
// memory or a TEXT column (SG-PT-08).
const MaxRequestBodyBytes = 1 << 20

// SecurityHeaders sets the response headers ZAP found missing on every
// service (SG-PT-09). The API serves JSON only, so nosniff costs nothing.
// CORP same-origin only affects no-cors loads (<img>, <script>); the
// mobile app's CORS fetches are unaffected.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// LimitRequestBody refuses a body over limit bytes with 413. A declared
// Content-Length is checked up front, so the upstream is never contacted;
// a chunked body is capped while it streams by http.MaxBytesReader, whose
// error the proxy turns into the same 413.
func LimitRequestBody(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit {
				httperror.Write(w, http.StatusRequestEntityTooLarge, "request body too large")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}
