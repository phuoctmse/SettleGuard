package api

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/phuoctmse/settleguard/gateway/internal/httperror"
	"github.com/phuoctmse/settleguard/gateway/internal/proxy"
)

// NewRouter assembles the middleware chain the spec fixes in §4:
//
//	RequestID -> CORS -> RateLimit(IP) -> APIKeyAuth -> RateLimit(client) -> Proxy
//
// CORS sits before auth because a browser preflight carries no key.
// /health is the only route outside the authenticated group.
func NewRouter(cfg Config, keys KeyLookup) (http.Handler, error) {
	upstreams := map[string]string{
		"/ledger":        cfg.LedgerUpstream,
		"/accounts":      cfg.AccountsUpstream,
		"/settlement":    cfg.SettlementUpstream,
		"/notifications": cfg.NotificationsUpstream,
	}
	parsed := make(map[string]*url.URL, len(upstreams))
	for prefix, raw := range upstreams {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("router: upstream for %s is not a valid URL: %q", prefix, raw)
		}
		parsed[prefix] = u
	}

	r := chi.NewRouter()
	r.Use(RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// go-chi/cors treats an EMPTY AllowedOrigins list as "allow every
	// origin" -- the exact opposite of the fail-closed behaviour the spec
	// requires. Deciding through AllowOriginFunc instead keeps the decision
	// ours: an empty allowlist matches nothing.
	allowed := make(map[string]struct{}, len(cfg.CORSAllowedOrigins))
	for _, o := range cfg.CORSAllowedOrigins {
		allowed[o] = struct{}{}
	}
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc: func(_ *http.Request, origin string) bool {
			_, ok := allowed[origin]
			return ok
		},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodOptions},
		AllowedHeaders:   []string{"Authorization", "Content-Type", HeaderRequestID},
		ExposedHeaders:   []string{HeaderRequestID},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(RateLimitByIP(NewRateLimiter(cfg.RateLimitIPPerMinute)))

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		httperror.Write(w, http.StatusNotFound, "not found")
	})

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	clientLimiter := NewRateLimiter(cfg.RateLimitClientPerMinute)
	r.Group(func(r chi.Router) {
		r.Use(APIKeyAuth(keys))
		r.Use(RateLimitByClient(clientLimiter))
		for prefix, u := range parsed {
			r.Mount(prefix, http.StripPrefix(prefix, proxy.New(u, cfg.UpstreamTimeout)))
		}
	})

	return r, nil
}
