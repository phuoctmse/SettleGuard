package account

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ClientStatus string

const (
	ClientStatusActive    ClientStatus = "active"
	ClientStatusSuspended ClientStatus = "suspended"
)

func (s ClientStatus) Valid() bool {
	switch s {
	case ClientStatusActive, ClientStatusSuspended:
		return true
	default:
		return false
	}
}

type ClientBusiness struct {
	ID        uuid.UUID
	Name      string
	Status    ClientStatus
	CreatedAt time.Time
}

var (
	ErrEmptyClientName     = errors.New("account: client name is required")
	ErrInvalidClientStatus = errors.New("account: invalid client status")
	ErrClientNotFound      = errors.New("account: client not found")
)

// NewClientBusiness builds a new ClientBusiness in the active status.
func NewClientBusiness(name string) (ClientBusiness, error) {
	if strings.TrimSpace(name) == "" {
		return ClientBusiness{}, ErrEmptyClientName
	}
	return ClientBusiness{
		ID:     uuid.New(),
		Name:   name,
		Status: ClientStatusActive,
	}, nil
}
