package account_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
)

func TestNewClientBusiness_Valid(t *testing.T) {
	client, err := account.NewClientBusiness("Acme Corp")

	assert.NoError(t, err)
	assert.Equal(t, "Acme Corp", client.Name)
	assert.Equal(t, account.ClientStatusActive, client.Status)
	assert.NotEqual(t, uuid.Nil, client.ID)
}

func TestNewClientBusiness_RejectsEmptyName(t *testing.T) {
	for _, name := range []string{"", "   "} {
		_, err := account.NewClientBusiness(name)
		assert.ErrorIs(t, err, account.ErrEmptyClientName)
	}
}

func TestClientStatus_Valid(t *testing.T) {
	tests := []struct {
		status account.ClientStatus
		want   bool
	}{
		{account.ClientStatusActive, true},
		{account.ClientStatusSuspended, true},
		{account.ClientStatus("bogus"), false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.status.Valid())
	}
}
