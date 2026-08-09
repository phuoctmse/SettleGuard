package api

import (
	"encoding/json"
	"net/http"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
)

const timeFormat = "2006-01-02T15:04:05.000Z07:00"

type Handlers struct {
	clients  *account.ClientRepository
	accounts *account.AccountRepository
}

func NewHandlers(clients *account.ClientRepository, accounts *account.AccountRepository) *Handlers {
	return &Handlers{clients: clients, accounts: accounts}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
