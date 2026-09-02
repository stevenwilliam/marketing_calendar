// Package ratelimit is an in-process token bucket, keyed by client.
//
// It protects login, TOTP and refresh (12-security.md §3). In-process is the
// right scope here: the service is a single binary behind nginx, and a shared
// store would add a dependency to the one path that must still work when
// everything else is failing.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   float64
	idleFor time.Duration
	lastGC  time.Time
	nowFn   func() time.Time
}

// New allows `burst` immediate attempts, refilling at `perMinute` per minute.
func New(burst int, perMinute float64) *Limiter {
	return &Limiter{
		buckets: map[string]*bucket{},
		rate:    perMinute / 60.0,
		burst:   float64(burst),
		idleFor: 30 * time.Minute,
		nowFn:   time.Now,
	}
}

// Allow consumes a token. It returns the wait before the next attempt when it
// refuses, so the handler can set Retry-After rather than leaving the client
// to guess.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.nowFn()
	l.gc(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, lastSeen: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.lastSeen).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.lastSeen = now

	if b.tokens < 1 {
		need := (1 - b.tokens) / l.rate
		return false, time.Duration(need * float64(time.Second))
	}
	b.tokens--
	return true, 0
}

// Reset clears one key, for a successful login.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// gc keeps the map from growing without bound: an attacker cycling source
// addresses would otherwise turn the limiter itself into the memory leak.
func (l *Limiter) gc(now time.Time) {
	if now.Sub(l.lastGC) < 5*time.Minute {
		return
	}
	l.lastGC = now
	for k, b := range l.buckets {
		if now.Sub(b.lastSeen) > l.idleFor {
			delete(l.buckets, k)
		}
	}
}
