package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
)

const timeFormat = "2006-01-02T15:04:05.000Z07:00"

type Handlers struct {
	transactions *settlement.TransactionRepository
	settlements  *settlement.SettlementRepository
}

func NewHandlers(transactions *settlement.TransactionRepository, settlements *settlement.SettlementRepository) *Handlers {
	return &Handlers{transactions: transactions, settlements: settlements}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Encode errors are not recoverable once headers are sent; client sees partial response either way.
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

type transactionResponse struct {
	ID             string   `json:"id"`
	AccountIDs     []string `json:"account_ids"`
	Amount         int64    `json:"amount"`
	Score          int      `json:"score"`
	Decision       string   `json:"decision"`
	Status         string   `json:"status"`
	TriggeredRules []string `json:"triggered_rules"`
	ScoredAt       string   `json:"scored_at"`
}

func toTransactionResponse(t settlement.Transaction) transactionResponse {
	accountIDs := make([]string, len(t.AccountIDs))
	for i, id := range t.AccountIDs {
		accountIDs[i] = id.String()
	}
	return transactionResponse{
		ID:             t.ID.String(),
		AccountIDs:     accountIDs,
		Amount:         t.Amount,
		Score:          t.Score,
		Decision:       t.Decision,
		Status:         t.Status,
		TriggeredRules: t.TriggeredRules,
		ScoredAt:       t.ScoredAt.Format(timeFormat),
	}
}

// ListTransactions requires a status filter -- the one caller (mobile-app's
// Held Transactions screen) always wants status=held; an unfiltered "list
// everything" endpoint has no use case yet and would need pagination
// nothing currently needs.
func (h *Handlers) ListTransactions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		writeError(w, http.StatusBadRequest, "status query param is required")
		return
	}

	txs, err := h.transactions.ListByStatus(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]transactionResponse, len(txs))
	for i, t := range txs {
		resp[i] = toTransactionResponse(t)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) GetTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}

	t, err := h.transactions.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, settlement.ErrTransactionNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, toTransactionResponse(*t))
}

func (h *Handlers) ApproveTransaction(w http.ResponseWriter, r *http.Request) {
	h.resolveTransaction(w, r, h.transactions.Approve)
}

func (h *Handlers) RejectTransaction(w http.ResponseWriter, r *http.Request) {
	h.resolveTransaction(w, r, h.transactions.Reject)
}

// resolveTransaction backs both ApproveTransaction and RejectTransaction --
// they differ only in which TransactionRepository method resolves the hold.
// See SETTLEMENT-05 in docs/BUSINESS_RULES.md for the status-transition rule
// this enforces via ErrTransactionNotHeld -> 409.
func (h *Handlers) resolveTransaction(
	w http.ResponseWriter,
	r *http.Request,
	resolve func(context.Context, uuid.UUID) (*settlement.Transaction, error),
) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}

	t, err := resolve(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, settlement.ErrTransactionNotFound):
			writeError(w, http.StatusNotFound, "transaction not found")
		case errors.Is(err, settlement.ErrTransactionNotHeld):
			writeError(w, http.StatusConflict, "transaction is not held")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, toTransactionResponse(*t))
}

type settlementResponse struct {
	ID               string   `json:"id"`
	TransactionIDs   []string `json:"transaction_ids"`
	TransactionCount int      `json:"transaction_count"`
	TotalAmount      int64    `json:"total_amount"`
	CreatedAt        string   `json:"created_at"`
}

func toSettlementResponse(s settlement.Settlement) settlementResponse {
	ids := make([]string, len(s.TransactionIDs))
	for i, id := range s.TransactionIDs {
		ids[i] = id.String()
	}
	return settlementResponse{
		ID:               s.ID.String(),
		TransactionIDs:   ids,
		TransactionCount: s.TransactionCount,
		TotalAmount:      s.TotalAmount,
		CreatedAt:        s.CreatedAt.Format(timeFormat),
	}
}

func (h *Handlers) ListSettlements(w http.ResponseWriter, r *http.Request) {
	list, err := h.settlements.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]settlementResponse, len(list))
	for i, s := range list {
		resp[i] = toSettlementResponse(s)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) GetSettlement(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid settlement id")
		return
	}

	s, err := h.settlements.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, settlement.ErrSettlementNotFound) {
			writeError(w, http.StatusNotFound, "settlement not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, toSettlementResponse(*s))
}
