package api

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
)

// Postgres timestamptz keeps microseconds; the cursor must carry them intact or
// keyset paging would skip or repeat rows at a page boundary.
func TestCursorRoundTripKeepsMicrosecondPrecision(t *testing.T) {
	at := time.Date(2026, 10, 18, 12, 0, 0, 123456000, time.UTC)
	id := uuid.New()

	enc := encodeCursor(&settlement.Cursor{At: at, ID: id})
	if enc == nil {
		t.Fatal("encodeCursor returned nil for a non-nil cursor")
	}
	if strings.Contains(*enc, "=") {
		t.Errorf("cursor %q has base64 padding", *enc)
	}

	req := httptest.NewRequest("GET", "/transactions?cursor="+url.QueryEscape(*enc), nil)
	got, err := parseCursor(req)
	if err != nil {
		t.Fatalf("parseCursor: %v", err)
	}
	if !got.At.Equal(at) {
		t.Errorf("At = %s, want %s", got.At.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano))
	}
	if got.ID != id {
		t.Errorf("ID = %s, want %s", got.ID, id)
	}
}
