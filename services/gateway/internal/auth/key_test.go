package auth_test

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phuoctmse/settleguard/gateway/internal/auth"
)

// sg_live_ + 43 base64url chars (256 bits, no padding) = 51 chars total.
var keyShape = regexp.MustCompile(`^sg_live_[A-Za-z0-9_-]{43}$`)

func TestGenerateKey_HasPrefixAnd256BitsOfBase64URL(t *testing.T) {
	raw, _, err := auth.GenerateKey()
	require.NoError(t, err)
	assert.Regexp(t, keyShape, raw)
	assert.Len(t, raw, len(auth.KeyPrefix)+43)
}

func TestGenerateKey_ReturnsHashOfTheRawKey(t *testing.T) {
	raw, hash, err := auth.GenerateKey()
	require.NoError(t, err)
	assert.Equal(t, auth.HashKey(raw), hash)
}

func TestGenerateKey_IsUniquePerCall(t *testing.T) {
	a, _, err := auth.GenerateKey()
	require.NoError(t, err)
	b, _, err := auth.GenerateKey()
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
}

func TestHashKey_IsSHA256Hex(t *testing.T) {
	// Known-answer against the standard library: the digest must be the
	// plain SHA-256 of the raw key, hex-encoded, so an operator can verify a
	// stored hash with `echo -n <key> | sha256sum` if they ever need to.
	sum := sha256.Sum256([]byte("sg_live_x"))
	assert.Equal(t, hex.EncodeToString(sum[:]), auth.HashKey("sg_live_x"))
	assert.Len(t, auth.HashKey("sg_live_x"), 64)
	assert.NotEqual(t, auth.HashKey("sg_live_x"), auth.HashKey("sg_live_y"))
}
