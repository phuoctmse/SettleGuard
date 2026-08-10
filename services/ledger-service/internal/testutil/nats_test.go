package testutil_test

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"

	"github.com/phuoctmse/settleguard/ledger-service/internal/testutil"
)

func TestNewTestNATS_ConnectsWithJetStream(t *testing.T) {
	conn, js := testutil.NewTestNATS(t)

	assert.Equal(t, nats.CONNECTED, conn.Status())
	assert.NotNil(t, js)
}
