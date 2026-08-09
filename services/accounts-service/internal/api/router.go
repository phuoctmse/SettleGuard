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

	return r
}
