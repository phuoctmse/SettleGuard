package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 200
)

var (
	errInvalidLimit  = errors.New("invalid limit")
	errInvalidCursor = errors.New("invalid cursor")
)

// pageResponse is the body of every list endpoint. Items is always a JSON
// array; NextCursor is null on the last page.
type pageResponse struct {
	Items      any     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

// cursorWire is the JSON inside the opaque base64url cursor. The timestamp
// keeps full precision -- the API's display format stops at milliseconds,
// which would skip or repeat rows scored within the same millisecond.
type cursorWire struct {
	T  string `json:"t"`
	ID string `json:"id"`
}

// parseLimit reads ?limit=: absent means the default, above the maximum is
// clamped, and anything below 1 or non-numeric is rejected (a lower bound the
// notification-service lacked -- SG-PT-07).
func parseLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultPageLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, errInvalidLimit
	}
	return min(n, maxPageLimit), nil
}

// parseCursor reads ?cursor=; absent means the first page.
func parseCursor(r *http.Request) (*settlement.Cursor, error) {
	raw := r.URL.Query().Get("cursor")
	if raw == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, errInvalidCursor
	}
	var wire cursorWire
	if err := json.Unmarshal(decoded, &wire); err != nil {
		return nil, errInvalidCursor
	}
	at, err := time.Parse(time.RFC3339Nano, wire.T)
	if err != nil {
		return nil, errInvalidCursor
	}
	id, err := uuid.Parse(wire.ID)
	if err != nil {
		return nil, errInvalidCursor
	}
	return &settlement.Cursor{At: at, ID: id}, nil
}

// encodeCursor turns the repository's next-page position into the opaque
// string clients send back; nil (no next page) stays nil, which encodes as null.
func encodeCursor(c *settlement.Cursor) *string {
	if c == nil {
		return nil
	}
	// Marshalling two strings cannot fail.
	b, _ := json.Marshal(cursorWire{T: c.At.UTC().Format(time.RFC3339Nano), ID: c.ID.String()})
	s := base64.RawURLEncoding.EncodeToString(b)
	return &s
}
