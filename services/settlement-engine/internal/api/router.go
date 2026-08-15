package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter returns settlement-engine's HTTP router. This MVP is
// event-driven end-to-end (consumer -> scorer -> repository -> outbox
// relay); the only HTTP surface is a health check for orchestration.
func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", health)

	return r
}

func health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Error is not recoverable after status is sent; client will see partial response either way.
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
