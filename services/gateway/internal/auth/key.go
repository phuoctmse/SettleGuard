// Package auth owns API keys: how they are generated, how they are stored,
// and how a presented key is resolved back to a client business.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// KeyPrefix marks every SettleGuard API key so one that leaks into a log or
// a repository is recognisable on sight.
const KeyPrefix = "sg_live_"

// keyBytes is the entropy of a key: 256 bits, which base64url encodes to 43
// characters with no padding.
const keyBytes = 32

// GenerateKey returns a fresh raw key and its hash. The raw key is shown to
// the operator exactly once, by adminctl; only the hash is ever stored.
func GenerateKey() (raw string, hash string, err error) {
	buf := make([]byte, keyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("auth: generate key: %w", err)
	}
	raw = KeyPrefix + base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashKey(raw), nil
}

// HashKey returns the SHA-256 hex digest of raw.
//
// SHA-256 rather than bcrypt or argon2 is deliberate. Those exist to slow
// down brute force against low-entropy, human-chosen passwords; a key here
// is 256 random bits, so their cost buys nothing. More importantly, bcrypt
// salts each row differently, which makes lookup-by-hash impossible and
// forces a full table scan on every request. A plain digest can sit behind
// a UNIQUE index and be found in O(1).
func HashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
