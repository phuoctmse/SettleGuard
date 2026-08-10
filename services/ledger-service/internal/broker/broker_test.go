package broker_test

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/ledger-service/internal/broker"
	"github.com/phuoctmse/settleguard/ledger-service/internal/testutil"
)

func TestConnect(t *testing.T) {
	url := testutil.NewTestNATSURL(t)

	conn, js, err := broker.Connect(url)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	assert.Equal(t, nats.CONNECTED, conn.Status())
	assert.NotNil(t, js)
}

func TestConnect_InvalidURL(t *testing.T) {
	_, _, err := broker.Connect("nats://127.0.0.1:1")
	assert.Error(t, err)
}

func TestEnsureStream_CreatesLedgerEventsStream(t *testing.T) {
	_, js := testutil.NewTestNATS(t)

	err := broker.EnsureStream(context.Background(), js)
	require.NoError(t, err)

	stream, err := js.Stream(context.Background(), broker.LedgerEventsStream)
	require.NoError(t, err)

	info, err := stream.Info(context.Background())
	require.NoError(t, err)
	assert.Contains(t, info.Config.Subjects, "ledger.>")
}

func TestEnsureStream_IsIdempotent(t *testing.T) {
	_, js := testutil.NewTestNATS(t)

	require.NoError(t, broker.EnsureStream(context.Background(), js))
	err := broker.EnsureStream(context.Background(), js)
	assert.NoError(t, err, "calling EnsureStream twice with the same config must not error")
}
