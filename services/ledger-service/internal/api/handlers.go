package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
)

const timeFormat = "2006-01-02T15:04:05.000Z07:00"

type Handlers struct {
	repo *ledger.Repository
}

type entryRequest struct {
	AccountID string `json:"account_id"`
	Direction string `json:"direction"`
	Amount    int64  `json:"amount"`
	Reason    string `json:"reason"`
}

type createTransactionRequest struct {
	Entries []entryRequest `json:"entries"`
}

type entryResponse struct {
	ID            string `json:"id"`
	TransactionID string `json:"transaction_id"`
	AccountID     string `json:"account_id"`
	Direction     string `json:"direction"`
	Amount        int64  `json:"amount"`
	Reason        string `json:"reason"`
	CreatedAt     string `json:"created_at"`
}

func NewHandlers(repo *ledger.Repository) *Handlers {
	return &Handlers{repo: repo}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Error is not recoverable after status is sent; client sees partial response either way.
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Encode errors are not recoverable once headers are sent; client sees partial response either way.
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func writeEntries(w http.ResponseWriter, status int, entries []ledger.Entry) {
	resp := make([]entryResponse, len(entries))
	for i, e := range entries {
		resp[i] = entryResponse{
			ID:            e.ID.String(),
			TransactionID: e.TransactionID.String(),
			AccountID:     e.AccountID.String(),
			Direction:     string(e.Direction),
			Amount:        e.Amount,
			Reason:        e.Reason,
			CreatedAt:     e.CreatedAt.Format(timeFormat),
		}
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	// Encode errors are not recoverable once headers are sent; client sees partial response either way.
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handlers) ListEntries(w http.ResponseWriter, r *http.Request) {
	accountParam := r.URL.Query().Get("account_id")
	transactionParam := r.URL.Query().Get("transaction_id")

	var (
		entries []ledger.Entry
		err     error
	)

	switch {
	case accountParam != "":
		id, parseErr := uuid.Parse(accountParam)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid account_id")
			return
		}
		entries, err = h.repo.ListByAccount(r.Context(), id)
	case transactionParam != "":
		id, parseErr := uuid.Parse(transactionParam)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid transaction_id")
			return
		}
		entries, err = h.repo.ListByTransaction(r.Context(), id)
	default:
		writeError(w, http.StatusBadRequest, "account_id or transaction_id query param is required")
		return
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeEntries(w, http.StatusOK, entries)
}

func (h *Handlers) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req createTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
	}

	entries := make([]ledger.Entry, 0, len(req.Entries))
	for _, e := range req.Entries {
		accountID, err := uuid.Parse(e.AccountID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid account_id")
			return
		}
		entries = append(entries, ledger.Entry{
			AccountID: accountID,
			Direction: ledger.Direction(e.Direction),
			Amount:    e.Amount,
			Reason:    e.Reason,
		})
	}

	inserted, err := h.repo.InsertTransaction(r.Context(), uuid.New(), entries)
	if err != nil {
		switch {
		case errors.Is(err, ledger.ErrUnbalancedTransaction),
			errors.Is(err, ledger.ErrInvalidAmount),
			errors.Is(err, ledger.ErrInvalidDirection),
			errors.Is(err, ledger.ErrNoEntries):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeEntries(w, http.StatusCreated, inserted)
}
