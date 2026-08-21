package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter returns settlement-engine's HTTP router.
func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.Health)

	r.Get("/transactions", h.ListTransactions)
	r.Get("/transactions/{id}", h.GetTransaction)
	r.Post("/transactions/{id}/approve", h.ApproveTransaction)
	r.Post("/transactions/{id}/reject", h.RejectTransaction)

	r.Get("/settlements", h.ListSettlements)
	r.Get("/settlements/{id}", h.GetSettlement)

	return r
}
