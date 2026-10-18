package main

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/gateway/internal/auth"
)

// runCreate issues a key and prints it. This is the only place the raw key
// is ever visible; the database keeps the hash and nothing else.
func runCreate(ctx context.Context, repo *auth.Repository, clientID uuid.UUID, label string, out io.Writer) error {
	key, raw, err := repo.Create(ctx, clientID, label)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "id:        %s\n", key.ID)
	fmt.Fprintf(out, "client_id: %s\n", key.ClientID)
	fmt.Fprintf(out, "label:     %s\n", key.Label)
	fmt.Fprintf(out, "key:       %s\n", raw)
	fmt.Fprintln(out, "Store the key now. It is shown once and cannot be recovered.")
	return nil
}

// runRevoke marks a key unusable; the row stays for audit.
func runRevoke(ctx context.Context, repo *auth.Repository, id uuid.UUID, out io.Writer) error {
	if err := repo.Revoke(ctx, id); err != nil {
		return err
	}
	fmt.Fprintf(out, "revoked: %s\n", id)
	return nil
}

// runList prints every key issued to a client. Raw keys are not stored, so
// there is nothing sensitive here to hide -- only ids, labels and dates.
func runList(ctx context.Context, repo *auth.Repository, clientID uuid.UUID, out io.Writer) error {
	keys, err := repo.List(ctx, clientID)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		fmt.Fprintf(out, "no keys for client %s\n", clientID)
		return nil
	}
	fmt.Fprintf(out, "%-36s  %-24s  %-20s  %s\n", "ID", "CREATED", "REVOKED", "LABEL")
	for _, k := range keys {
		revoked := "-"
		if k.RevokedAt != nil {
			revoked = k.RevokedAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		fmt.Fprintf(out, "%-36s  %-24s  %-20s  %s\n", k.ID, k.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"), revoked, k.Label)
	}
	return nil
}
