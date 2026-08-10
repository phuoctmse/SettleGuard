package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/phuoctmse/settleguard/accounts-service/internal/account"
	"github.com/phuoctmse/settleguard/accounts-service/internal/api"
	"github.com/phuoctmse/settleguard/accounts-service/internal/broker"
	"github.com/phuoctmse/settleguard/accounts-service/internal/consumer"
	"github.com/phuoctmse/settleguard/accounts-service/internal/db"
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

	if err := broker.EnsureStream(ctx, js); err != nil {
		log.Fatalf("ensure jetstream stream: %v", err)
	}

	clients := account.NewClientRepository(conn)
	accounts := account.NewAccountRepository(conn)

	balanceConsumer := consumer.New(accounts)
	consumeCtx, err := balanceConsumer.Start(ctx, js)
	if err != nil {
		log.Fatalf("start balance consumer: %v", err)
	}
	defer consumeCtx.Stop()

	handlers := api.NewHandlers(clients, accounts)
	router := api.NewRouter(handlers)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8081"
	}

	log.Printf("accounts-service listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}
