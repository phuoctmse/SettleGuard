package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
)

type accountResponse struct {
	ID          string `json:"id"`
	ClientID    string `json:"client_id"`
	ExternalRef string `json:"external_ref,omitempty"`
	Status      string `json:"status"`
	Balance     int64  `json:"balance"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toAccountResponse(a account.Account) accountResponse {
	return accountResponse{
		ID:          a.ID.String(),
		ClientID:    a.ClientID.String(),
		ExternalRef: a.ExternalRef,
		Status:      string(a.Status),
		Balance:     a.Balance,
		CreatedAt:   a.CreatedAt.Format(timeFormat),
		UpdatedAt:   a.UpdatedAt.Format(timeFormat),
	}
}

type createAccountRequest struct {
	ClientID    string `json:"client_id"`
	ExternalRef string `json:"external_ref"`
}

func (h *Handlers) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	clientID, err := uuid.Parse(req.ClientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid client_id")
		return
	}

	acc, err := h.accounts.Create(r.Context(), clientID, req.ExternalRef)
	if err != nil {
		switch {
		case errors.Is(err, account.ErrClientNotFound):
			writeError(w, http.StatusNotFound, "client not found")
		case errors.Is(err, account.ErrClientSuspended):
			writeError(w, http.StatusUnprocessableEntity, "client is suspended")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, toAccountResponse(acc))
}

func (h *Handlers) GetAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	acc, err := h.accounts.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, account.ErrAccountNotFound) {
			writeError(w, http.StatusNotFound, "account not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, toAccountResponse(acc))
}

func (h *Handlers) ListAccounts(w http.ResponseWriter, r *http.Request) {
	clientParam := r.URL.Query().Get("client_id")
	if clientParam == "" {
		writeError(w, http.StatusBadRequest, "client_id query param is required")
		return
	}

	clientID, err := uuid.Parse(clientParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid client_id")
		return
	}

	accounts, err := h.accounts.ListByClient(r.Context(), clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]accountResponse, len(accounts))
	for i, a := range accounts {
		resp[i] = toAccountResponse(a)
	}
	writeJSON(w, http.StatusOK, resp)
}

type updateAccountStatusRequest struct {
	Status string `json:"status"`
}

func (h *Handlers) UpdateAccountStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	var req updateAccountStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	acc, err := h.accounts.UpdateStatus(r.Context(), id, account.AccountStatus(req.Status))
	if err != nil {
		switch {
		case errors.Is(err, account.ErrInvalidAccountStatus):
			writeError(w, http.StatusBadRequest, "invalid status")
		case errors.Is(err, account.ErrAccountNotFound):
			writeError(w, http.StatusNotFound, "account not found")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, toAccountResponse(acc))
}
