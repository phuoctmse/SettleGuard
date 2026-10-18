// Command server runs the SettleGuard gateway: the single entry point in
// front of the four backend services, providing CORS, API-key
// authentication, rate limiting and request ids.
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

	"github.com/phuoctmse/settleguard/gateway/internal/api"
	"github.com/phuoctmse/settleguard/gateway/internal/auth"
	"github.com/phuoctmse/settleguard/gateway/internal/db"
)

// HTTP server timeouts. Identical to the three backend Go services so every
// SettleGuard process behaves the same under slow or stalled clients.
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
	cfg, err := api.LoadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer conn.Close()

	if err := db.Migrate(conn); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	router, err := api.NewRouter(cfg, auth.NewRepository(conn))
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serverErr := make(chan error, 1)
	log.Printf("gateway listening on %s", cfg.ListenAddr)
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
