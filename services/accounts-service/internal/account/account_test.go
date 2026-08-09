package account_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
)

func TestNewAccount_Defaults(t *testing.T) {
	clientID := uuid.New()
	acc := account.NewAccount(clientID, "ext-123")

	assert.Equal(t, clientID, acc.ClientID)
	assert.Equal(t, "ext-123", acc.ExternalRef)
	assert.Equal(t, account.AccountStatusActive, acc.Status)
	assert.NotEqual(t, uuid.Nil, acc.ID)
}

func TestAccountStatus_Valid(t *testing.T) {
	tests := []struct {
		status account.AccountStatus
		want   bool
	}{
		{account.AccountStatusActive, true},
		{account.AccountStatusSuspended, true},
		{account.AccountStatusClosed, true},
		{account.AccountStatus("bogus"), false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.status.Valid())
	}
}

func TestCanCreateAccount(t *testing.T) {
	tests := []struct {
		name         string
		clientStatus account.ClientStatus
		want         bool
	}{
		{"active client allows creation", account.ClientStatusActive, true},
		{"suspended client blocks creation", account.ClientStatusSuspended, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, account.CanCreateAccount(tt.clientStatus))
		})
	}
}
