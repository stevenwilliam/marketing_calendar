package ratelimit

import (
	"testing"
	"time"
)

func TestBurstThenRefuse(t *testing.T) {
	l := New(3, 60)
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("1.2.3.4"); !ok {
			t.Fatalf("attempt %d must be allowed within the burst", i+1)
		}
	}
	ok, wait := l.Allow("1.2.3.4")
	if ok {
		t.Fatal("the fourth attempt must be refused")
	}
	if wait <= 0 {
		t.Fatal("a refusal must say how long to wait, for Retry-After")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l := New(1, 60)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal()
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("one client's limit must not affect another's")
	}
}

func TestRefill(t *testing.T) {
	now := time.Now()
	l := New(1, 60) // one per second
	l.nowFn = func() time.Time { return now }
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal()
	}
	if ok, _ := l.Allow("k"); ok {
		t.Fatal("immediately after, it must refuse")
	}
	now = now.Add(2 * time.Second)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("after the refill window it must allow again")
	}
}

// An attacker cycling source addresses must not turn the limiter into the
// memory leak that takes the service down.
func TestIdleBucketsAreCollected(t *testing.T) {
	now := time.Now()
	l := New(1, 60)
	l.nowFn = func() time.Time { return now }
	for i := 0; i < 1000; i++ {
		l.Allow(string(rune(i)) + "-key")
	}
	if len(l.buckets) != 1000 {
		t.Fatalf("expected 1000 buckets, got %d", len(l.buckets))
	}
	now = now.Add(2 * time.Hour)
	l.Allow("trigger-gc")
	if len(l.buckets) > 2 {
		t.Fatalf("idle buckets were not collected: %d remain", len(l.buckets))
	}
}

func TestResetOnSuccess(t *testing.T) {
	l := New(1, 1)
	l.Allow("k")
	if ok, _ := l.Allow("k"); ok {
		t.Fatal()
	}
	l.Reset("k")
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("a successful login must clear the client's failures")
	}
}
