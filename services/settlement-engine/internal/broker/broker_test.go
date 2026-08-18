package broker_test

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/broker"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/testutil"
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

func TestEnsureStream_CreatesStream(t *testing.T) {
	_, js := testutil.NewTestNATS(t)

	cfg := jetstream.StreamConfig{
		Name:     broker.LedgerEventsStream,
		Subjects: []string{"ledger.>"},
		Storage:  jetstream.FileStorage,
	}
	err := broker.EnsureStream(context.Background(), js, cfg)
	require.NoError(t, err)

	stream, err := js.Stream(context.Background(), broker.LedgerEventsStream)
	require.NoError(t, err)

	info, err := stream.Info(context.Background())
	require.NoError(t, err)
	assert.Contains(t, info.Config.Subjects, "ledger.>")
}

func TestEnsureStream_IsIdempotent(t *testing.T) {
	_, js := testutil.NewTestNATS(t)

	cfg := jetstream.StreamConfig{
		Name:     broker.SettlementEventsStream,
		Subjects: []string{"settlement.>", "transaction.risk-scored"},
		Storage:  jetstream.FileStorage,
	}
	require.NoError(t, broker.EnsureStream(context.Background(), js, cfg))
	err := broker.EnsureStream(context.Background(), js, cfg)
	assert.NoError(t, err, "calling EnsureStream twice with the same config must not error")
}
