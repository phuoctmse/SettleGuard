package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/phuoctmse/settleguard/ledger-service/internal/api"
	"github.com/phuoctmse/settleguard/ledger-service/internal/broker"
	"github.com/phuoctmse/settleguard/ledger-service/internal/db"
	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
	"github.com/phuoctmse/settleguard/ledger-service/internal/outbox"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment")
	}
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		log.Fatal("NATS_URL environment")
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

	if err := broker.EnsureStream(ctx, js); err != nil {
		log.Fatalf("ensure jetstream stream: %v", err)
	}

	relay := outbox.NewRelay(conn, js)
	go func() {
		if err := relay.Run(ctx); err != nil {
			log.Printf("outbox relay stopped: %v", err)
		}
	}()

	repo := ledger.NewRepository(conn)
	handlers := api.NewHandlers(repo)
	router := api.NewRouter(handlers)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("ledger-service listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}
