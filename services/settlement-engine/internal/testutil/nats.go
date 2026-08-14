package testutil

import (
	"context"
	"testing"

	natstc "github.com/testcontainers/testcontainers-go/modules/nats"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NewTestNATSURL starts a throwaway NATS server with JetStream enabled and
// returns its client connection URL. The container is torn down
// automatically when the test completes.
func NewTestNATSURL(t testing.TB) string {
	t.Helper()
	ctx := context.Background()

	// The module already passes "-js" by default (JetStream on) — no extra
	// CmdOption needed. Passing WithArgument("js", "") on top breaks
	// nats-server's arg parsing (it renders as a bare empty-string arg).
	container, err := natstc.Run(ctx, "nats:2.10-alpine")
	if err != nil {
		t.Fatalf("start nats container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	url, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("get nats connection string: %v", err)
	}

	return url
}

// NewTestNATS starts a throwaway NATS server with JetStream enabled and
// returns a connected *nats.Conn plus its JetStream handle. The container
// and connection are torn down automatically when the test completes.
func NewTestNATS(t testing.TB) (*nats.Conn, jetstream.JetStream) {
	t.Helper()

	conn, err := nats.Connect(NewTestNATSURL(t))
	if err != nil {
		t.Fatalf("connect to test nats: %v", err)
	}
	t.Cleanup(conn.Close)

	js, err := jetstream.New(conn)
	if err != nil {
		t.Fatalf("create jetstream context: %v", err)
	}

	return conn, js
}
