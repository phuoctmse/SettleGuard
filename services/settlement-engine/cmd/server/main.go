package main

import (
	"context"
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
		Subjects: []string{"settlement.>", "transaction.risk-scored"},
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

	router := api.NewRouter()

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8082"
	}

	log.Printf("settlement-engine listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
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
