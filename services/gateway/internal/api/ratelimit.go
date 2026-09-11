package api

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/phuoctmse/settleguard/gateway/internal/httperror"
)

// sweepEvery is how many bucket creations happen between sweeps of idle
// buckets. See RateLimiter for the bound this actually buys and the one it
// does not.
const sweepEvery = 1000

// RateLimiter hands out one token bucket per key. It is in-memory and
// therefore correct for a single gateway instance only: two replicas each
// enforce the configured limit independently, so the effective limit
// doubles. Documented in the README; a shared store (Redis) is the fix
// when the gateway is scaled out.
//
// The map is bounded only while new keys arrive slower than roughly
// sweepEvery * burst / 60 requests per second across distinct addresses:
// below that rate, buckets refill to full before the next sweep and are
// dropped; a sustained flood of distinct source addresses above it keeps
// buckets partially spent and the map grows for the duration of the flood.
// Residual per-IP throttling against that belongs at the load balancer or
// WAF, outside the gateway. A full bucket is exactly the state a new
// bucket starts in, so evicting it is invisible to the caller.
// The IP tier runs before authentication, so without this an attacker could
// mint an unbounded number of buckets simply by varying the source address.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
	every   rate.Limit
	burst   int
	inserts int
}

// NewRateLimiter allows perMinute requests per key, with a burst of the
// same size so a client that was idle can spend its whole minute at once.
// perMinute must be positive; the configuration layer rejects zero and
// negatives, so a violation here is a programming error, not bad input.
func NewRateLimiter(perMinute int) *RateLimiter {
	if perMinute <= 0 {
		panic("api: NewRateLimiter: perMinute must be positive")
	}
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
		l.inserts++
		if l.inserts >= sweepEvery {
			l.inserts = 0
			l.sweepLocked()
		}
	}
	l.mu.Unlock()
	return b.Allow()
}

// sweepLocked drops every bucket that has refilled to full. The caller
// holds l.mu. A bucket removed here may still be referenced by the Allow
// call that triggered the sweep; that is fine -- the pointer stays valid
// and the next request for that key simply starts a fresh, equally full
// bucket.
func (l *RateLimiter) sweepLocked() {
	for k, b := range l.buckets {
		if b.Tokens() >= float64(l.burst) {
			delete(l.buckets, k)
		}
	}
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
				httperror.Write(w, http.StatusTooManyRequests, rateLimitedMessage)
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
				httperror.Write(w, http.StatusInternalServerError, "internal error")
				return
			}
			if !l.Allow(clientID.String()) {
				httperror.Write(w, http.StatusTooManyRequests, rateLimitedMessage)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
