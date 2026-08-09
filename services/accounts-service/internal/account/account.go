package account

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type AccountStatus string

const (
	AccountStatusActive    AccountStatus = "active"
	AccountStatusSuspended AccountStatus = "suspended"
	AccountStatusClosed    AccountStatus = "closed"
)

func (s AccountStatus) Valid() bool {
	switch s {
	case AccountStatusActive, AccountStatusSuspended, AccountStatusClosed:
		return true
	default:
		return false
	}
}

type Account struct {
	ID          uuid.UUID
	ClientID    uuid.UUID
	ExternalRef string
	Status      AccountStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

var (
	ErrInvalidAccountStatus = errors.New("account: invalid account status")
	ErrClientSuspended      = errors.New("account: cannot create account under a suspended client")
	ErrAccountNotFound      = errors.New("account: account not found")
)

// NewAccount builds a new Account in the active status under clientID.
func NewAccount(clientID uuid.UUID, externalRef string) Account {
	return Account{
		ID:          uuid.New(),
		ClientID:    clientID,
		ExternalRef: externalRef,
		Status:      AccountStatusActive,
	}
}

// CanCreateAccount reports whether a new Account may be created under a
// ClientBusiness currently in clientStatus. This is the one MVP rule that
// reaches into balance-of-obligation territory (spec §1) — both this
// function and its callers (AccountRepository.Create, the CreateAccount
// handler) require manual review before merge, not just passing tests.
func CanCreateAccount(clientStatus ClientStatus) bool {
	return clientStatus == ClientStatusActive
}
