package outbox_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/broker"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/outbox"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/testutil"
)

func insertOutboxRow(t *testing.T, db *sql.DB, subject string, payload []byte) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.Exec(`
		INSERT INTO outbox_events (id, event_type, subject, payload)
		VALUES ($1, $2, $3, $4)
	`, id, subject, subject, payload)
	require.NoError(t, err)
	return id
}

func ensureSettlementStream(t *testing.T, ctx context.Context, js jetstream.JetStream) {
	t.Helper()
	require.NoError(t, broker.EnsureStream(ctx, js, jetstream.StreamConfig{
		Name:     broker.SettlementEventsStream,
		Subjects: []string{"settlement.>", "transaction.risk-scored"},
		Storage:  jetstream.FileStorage,
	}))
}

func TestRelay_PublishBatch_PublishesAndMarksRow(t *testing.T) {
	db := testutil.NewTestDB(t)
	_, js := testutil.NewTestNATS(t)
	ctx := context.Background()

	ensureSettlementStream(t, ctx, js)
	consumer, err := js.CreateOrUpdateConsumer(ctx, broker.SettlementEventsStream, jetstream.ConsumerConfig{
		Durable:       "test-consumer",
		FilterSubject: "transaction.risk-scored",
	})
	require.NoError(t, err)

	id := insertOutboxRow(t, db, "transaction.risk-scored", []byte(`{"foo":"bar"}`))

	relay := outbox.NewRelay(db, js)
	n, err := relay.PublishBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	var publishedAt sql.NullTime
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT published_at FROM outbox_events WHERE id = $1
	`, id).Scan(&publishedAt))
	assert.True(t, publishedAt.Valid, "published_at should be set after a successful publish")

	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(3*time.Second))
	require.NoError(t, err)

	var got jetstream.Msg
	for msg := range batch.Messages() {
		got = msg
	}
	require.NoError(t, batch.Error())
	require.NotNil(t, got, "expected the relay to have published a message to the stream")

	assert.JSONEq(t, `{"foo":"bar"}`, string(got.Data()))
	assert.Equal(t, id.String(), got.Headers().Get(jetstream.MsgIDHeader))
}

func TestRelay_PublishBatch_LeavesUnpublishedRowsForNextCall(t *testing.T) {
	db := testutil.NewTestDB(t)
	_, js := testutil.NewTestNATS(t)
	ctx := context.Background()
	ensureSettlementStream(t, ctx, js)

	relay := outbox.NewRelay(db, js)
	n, err := relay.PublishBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "no rows pending, nothing to publish")
}

func TestRelay_Run_StopsOnContextCancel(t *testing.T) {
	db := testutil.NewTestDB(t)
	_, js := testutil.NewTestNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	relay := outbox.NewRelay(db, js)
	err := relay.Run(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
