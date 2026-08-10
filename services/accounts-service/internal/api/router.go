package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.Health)

	r.Post("/clients", h.CreateClient)
	r.Get("/clients/{id}", h.GetClient)
	r.Patch("/clients/{id}/status", h.UpdateClientStatus)

	r.Post("/accounts", h.CreateAccount)
	r.Get("/accounts", h.ListAccounts)
	r.Get("/accounts/{id}", h.GetAccount)
	r.Patch("/accounts/{id}/status", h.UpdateAccountStatus)

	return r
}
