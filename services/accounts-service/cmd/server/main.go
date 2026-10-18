package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
	"github.com/phuoctmse/settleguard/accounts-service/internal/api"
	"github.com/phuoctmse/settleguard/accounts-service/internal/broker"
	"github.com/phuoctmse/settleguard/accounts-service/internal/consumer"
	"github.com/phuoctmse/settleguard/accounts-service/internal/db"
	"github.com/phuoctmse/settleguard/accounts-service/internal/outbox"
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

	if err := broker.EnsureStream(ctx, js, jetstream.StreamConfig{
		Name:     broker.LedgerEventsStream,
		Subjects: []string{"ledger.>"},
		Storage:  jetstream.FileStorage,
	}); err != nil {
		log.Fatalf("ensure ledger events stream: %v", err)
	}
	if err := broker.EnsureStream(ctx, js, jetstream.StreamConfig{
		Name:     broker.AccountsEventsStream,
		Subjects: []string{"account.>"},
		Storage:  jetstream.FileStorage,
	}); err != nil {
		log.Fatalf("ensure accounts events stream: %v", err)
	}

	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)

	balanceConsumer := consumer.New(accounts)
	consumeCtx, err := balanceConsumer.Start(ctx, js)
	if err != nil {
		log.Fatalf("start balance consumer: %v", err)
	}
	defer consumeCtx.Stop()

	accountUpdatedRelay := outbox.NewRelay(conn, js)
	go func() {
		if err := accountUpdatedRelay.Run(ctx); err != nil {
			log.Printf("account.updated outbox relay stopped: %v", err)
		}
	}()

	handlers := api.NewHandlers(clients, accounts)
	router := api.NewRouter(handlers)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8081"
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
	log.Printf("accounts-service listening on %s", addr)
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
