// Command adminctl issues, revokes and lists gateway API keys by writing
// directly to the gateway's database. There is deliberately no HTTP
// endpoint for this: an open one would let anyone mint keys, and a
// protected one would need an admin identity the system does not have.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/gateway/internal/auth"
	"github.com/phuoctmse/settleguard/gateway/internal/db"
)

const usage = `usage:
  adminctl create --client-id <uuid> --label <text>
  adminctl revoke --id <uuid>
  adminctl list   --client-id <uuid>

DATABASE_URL must point at the gateway's Postgres.`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}
	conn, err := db.Connect(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect to db: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	if err := db.Migrate(conn); err != nil {
		fmt.Fprintf(os.Stderr, "run migrations: %v\n", err)
		os.Exit(1)
	}
	repo := auth.NewRepository(conn)
	ctx := context.Background()

	switch os.Args[1] {
	case "create":
		fs := flag.NewFlagSet("create", flag.ExitOnError)
		clientID := fs.String("client-id", "", "ClientBusiness id the key belongs to")
		label := fs.String("label", "", "human label, e.g. \"mobile-app prod\"")
		_ = fs.Parse(os.Args[2:])
		id, err := uuid.Parse(*clientID)
		if err != nil || *label == "" {
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(2)
		}
		err = runCreate(ctx, repo, id, *label, os.Stdout)
		exitOn(err)
	case "revoke":
		fs := flag.NewFlagSet("revoke", flag.ExitOnError)
		keyID := fs.String("id", "", "api key id to revoke")
		_ = fs.Parse(os.Args[2:])
		id, err := uuid.Parse(*keyID)
		if err != nil {
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(2)
		}
		exitOn(runRevoke(ctx, repo, id, os.Stdout))
	case "list":
		fs := flag.NewFlagSet("list", flag.ExitOnError)
		clientID := fs.String("client-id", "", "ClientBusiness id to list keys for")
		_ = fs.Parse(os.Args[2:])
		id, err := uuid.Parse(*clientID)
		if err != nil {
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(2)
		}
		exitOn(runList(ctx, repo, id, os.Stdout))
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
}

func exitOn(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
