package api

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter hands out one token bucket per key. It is in-memory and
// therefore correct for a single gateway instance only: two replicas each
// enforce the configured limit independently, so the effective limit
// doubles. Documented in the README; a shared store (Redis) is the fix
// when the gateway is scaled out.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
	every   rate.Limit
	burst   int
}

// NewRateLimiter allows perMinute requests per key, with a burst of the
// same size so a client that was idle can spend its whole minute at once.
func NewRateLimiter(perMinute int) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*rate.Limiter),
		every:   rate.Every(time.Minute / time.Duration(perMinute)),
		burst:   perMinute,
	}
}

// Allow reports whether key may make one more request right now.
func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	b, ok := l.buckets[key]
	if !ok {
		b = rate.NewLimiter(l.every, l.burst)
		l.buckets[key] = b
	}
	l.mu.Unlock()
	return b.Allow()
}

const rateLimitedMessage = "rate limit exceeded"

// RateLimitByIP is the first, pre-auth tier: it bounds a caller who has no
// key at all, so key guessing is throttled before it ever reaches the
// database. Keyed on the remote IP without the source port.
func RateLimitByIP(l *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			if !l.Allow(host) {
				writeError(w, http.StatusTooManyRequests, rateLimitedMessage)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitByClient is the second, post-auth tier, keyed on the client_id
// APIKeyAuth resolved. It must be mounted after APIKeyAuth; a request that
// reaches it without a client id is a wiring bug and is refused with 500
// rather than quietly sharing one bucket across every caller.
func RateLimitByClient(l *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID, ok := ClientIDFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if !l.Allow(clientID.String()) {
				writeError(w, http.StatusTooManyRequests, rateLimitedMessage)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
