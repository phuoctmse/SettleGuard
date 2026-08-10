package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
)

type clientResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func toClientResponse(c account.ClientBusiness) clientResponse {
	return clientResponse{
		ID:        c.ID.String(),
		Name:      c.Name,
		Status:    string(c.Status),
		CreatedAt: c.CreatedAt.Format(timeFormat),
	}
}

type createClientRequest struct {
	Name string `json:"name"`
}

func (h *Handlers) CreateClient(w http.ResponseWriter, r *http.Request) {
	var req createClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	client, err := h.clients.Create(r.Context(), req.Name)
	if err != nil {
		if errors.Is(err, account.ErrEmptyClientName) {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, toClientResponse(client))
}

func (h *Handlers) GetClient(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid client id")
		return
	}

	client, err := h.clients.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, account.ErrClientNotFound) {
			writeError(w, http.StatusNotFound, "client not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, toClientResponse(client))
}

type updateClientStatusRequest struct {
	Status string `json:"status"`
}

func (h *Handlers) UpdateClientStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid client id")
		return
	}

	var req updateClientStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	client, err := h.clients.UpdateStatus(r.Context(), id, account.ClientStatus(req.Status))
	if err != nil {
		switch {
		case errors.Is(err, account.ErrInvalidClientStatus):
			writeError(w, http.StatusBadRequest, "invalid status")
		case errors.Is(err, account.ErrClientNotFound):
			writeError(w, http.StatusNotFound, "client not found")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, toClientResponse(client))
}
