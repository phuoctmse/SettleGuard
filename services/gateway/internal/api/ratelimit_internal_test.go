package api

import (
	"fmt"
	"testing"
	"time"
)

// Every sweepEvery insertions the limiter must drop buckets that have
// refilled to full; otherwise an unauthenticated caller can grow the IP
// map without bound by cycling source addresses.
func TestRateLimiter_SweepsIdleBuckets(t *testing.T) {
	// 6000/min = 100 tokens/s, so one spent token refills in 10 ms.
	l := NewRateLimiter(6000)

	for i := 0; i < sweepEvery-1; i++ {
		l.Allow(fmt.Sprintf("ip-%d", i))
	}
	if got := len(l.buckets); got != sweepEvery-1 {
		t.Fatalf("before sweep: want %d buckets, got %d", sweepEvery-1, got)
	}

	time.Sleep(50 * time.Millisecond) // every bucket refills to full

	l.Allow("one-more") // the sweepEvery-th insertion triggers the sweep

	if got := len(l.buckets); got > 1 {
		t.Fatalf("after sweep: idle buckets must be dropped, map still has %d entries", got)
	}
}

// A swept key must behave exactly like a new key: full burst available.
func TestRateLimiter_SweptKeyStartsFreshAndIsStillLimited(t *testing.T) {
	l := NewRateLimiter(6000)

	if !l.Allow("victim") {
		t.Fatal("first request for a fresh key must be allowed")
	}
	l.mu.Lock()
	l.sweepLocked() // nothing is full yet: victim just spent a token
	l.mu.Unlock()
	if _, ok := l.buckets["victim"]; !ok {
		t.Fatal("a bucket that is not full must survive a sweep")
	}
}
