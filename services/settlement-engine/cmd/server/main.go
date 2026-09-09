package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/api"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/broker"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/consumer"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/db"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/outbox"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/risk"
	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
)

// HTTP server timeouts. Kept identical across the three SettleGuard Go
// services so all of them behave the same under slow or stalled clients.
const (
	// readHeaderTimeout bounds how long a client may take to send request
	// headers -- the direct guard against Slowloris-style connection holding.
	readHeaderTimeout = 5 * time.Second
	// readTimeout bounds reading the whole request, headers plus body.
	readTimeout = 15 * time.Second
	// writeTimeout bounds writing the response, so a stuck client cannot pin
	// a handler goroutine open indefinitely.
	writeTimeout = 30 * time.Second
	// idleTimeout bounds how long an idle keep-alive connection is kept.
	idleTimeout = 120 * time.Second
	// shutdownGrace bounds how long shutdown waits for in-flight requests to
	// finish before the process exits anyway.
	shutdownGrace = 15 * time.Second
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		log.Fatal("NATS_URL environment variable is required")
	}

	conn, err := db.Connect(dsn)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer conn.Close()

	if err := db.Migrate(conn); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	natsConn, js, err := broker.Connect(natsURL)
	if err != nil {
		log.Fatalf("connect to nats: %v", err)
	}
	defer natsConn.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// LedgerEventsStream is owned by ledger-service; ensuring it here too
	// (CreateOrUpdateStream is idempotent) removes a startup-ordering
	// dependency between the two services in local dev.
	if err := broker.EnsureStream(ctx, js, jetstream.StreamConfig{
		Name:     broker.LedgerEventsStream,
		Subjects: []string{"ledger.>"},
		Storage:  jetstream.FileStorage,
	}); err != nil {
		log.Fatalf("ensure ledger events stream: %v", err)
	}
	if err := broker.EnsureStream(ctx, js, jetstream.StreamConfig{
		Name:     broker.SettlementEventsStream,
		Subjects: []string{"settlement.>", "transaction.risk-scored", "transaction.resolved"},
		Storage:  jetstream.FileStorage,
	}); err != nil {
		log.Fatalf("ensure settlement events stream: %v", err)
	}

	transactions := settlement.NewTransactionRepository(conn)
	settlements := settlement.NewSettlementRepository(conn)

	// transactions satisfies both risk.VelocityLimiter and
	// risk.BlocklistChecker structurally -- one repo, two roles.
	scorer := risk.NewScorer(riskConfigFromEnv(), transactions, transactions)

	riskConsumer := consumer.New(scorer, transactions)
	consumeCtx, err := riskConsumer.Start(ctx, js)
	if err != nil {
		log.Fatalf("start risk-scoring consumer: %v", err)
	}
	defer consumeCtx.Stop()

	scheduler := settlement.NewScheduler(settlements, batchIntervalFromEnv())
	go func() {
		if err := scheduler.Run(ctx); err != nil {
			log.Printf("settlement scheduler stopped: %v", err)
		}
	}()

	relay := outbox.NewRelay(conn, js)
	go func() {
		if err := relay.Run(ctx); err != nil {
			log.Printf("outbox relay stopped: %v", err)
		}
	}()

	handlers := api.NewHandlers(transactions, settlements)
	router := api.NewRouter(handlers)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8082"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serverErr := make(chan error, 1)
	log.Printf("settlement-engine listening on %s", addr)
	go func() {
		// ErrServerClosed is the normal result of srv.Shutdown, not a failure.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("http server: %v", err)
		}
	case <-ctx.Done():
		log.Print("shutdown signal received, draining http server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("http server shutdown: %v", err)
		} else {
			log.Print("http server shut down cleanly")
		}
	}
}

func riskConfigFromEnv() risk.Config {
	return risk.Config{
		VelocityLimit:     envInt("SETTLEMENT_VELOCITY_LIMIT", 5),
		VelocityWindow:    time.Duration(envInt("SETTLEMENT_VELOCITY_WINDOW_MINUTES", 5)) * time.Minute,
		MismatchThreshold: int64(envInt("SETTLEMENT_MISMATCH_THRESHOLD", 10_000_000)),
	}
}

func batchIntervalFromEnv() time.Duration {
	return time.Duration(envInt("SETTLEMENT_BATCH_INTERVAL_SECONDS", 60)) * time.Second
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		log.Fatalf("%s must be an integer, got %q: %v", key, raw, err)
	}
	return v
}
