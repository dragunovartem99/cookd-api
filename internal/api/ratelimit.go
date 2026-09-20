package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// limiter throttles callers with one token bucket per key (a client IP or an
// account). Buckets refill continuously rather than in fixed windows, so a
// caller who spends their allowance regains it gradually instead of at the top
// of a minute.
//
// The state is per-process, which suits the single-container deployment. Give
// the API a second replica and this has to move to a shared store.
type limiter struct {
	refill rate.Limit
	burst  int
	// idle is how long a silent caller's bucket is kept. It is the window the
	// limiter was built with, i.e. long enough for a full refill.
	idle time.Duration
	// retryAfter is how many seconds one token takes to return.
	retryAfter string

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	tokens *rate.Limiter
	seen   time.Time
}

// newLimiter allows requests bursts of the given size, refilled over window.
func newLimiter(requests int, window time.Duration) *limiter {
	perToken := window / time.Duration(requests)
	return &limiter{
		refill:     rate.Every(perToken),
		burst:      requests,
		idle:       window,
		retryAfter: strconv.Itoa(max(int(perToken.Round(time.Second)/time.Second), 1)),
		buckets:    make(map[string]*bucket),
		lastSweep:  time.Now(),
	}
}

// limit is the middleware form: it buckets requests by key(r) and rejects
// throttled callers with the documented error body and a Retry-After hint.
func (l *limiter) limit(key func(*http.Request) string) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.allow(key(r)) {
				w.Header().Set("Retry-After", l.retryAfter)
				writeError(w, http.StatusTooManyRequests, "Rate limit exceeded, please try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *limiter) allow(key string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweep(now)

	visitor, ok := l.buckets[key]
	if !ok {
		visitor = &bucket{tokens: rate.NewLimiter(l.refill, l.burst)}
		l.buckets[key] = visitor
	}
	visitor.seen = now

	return visitor.tokens.AllowN(now, 1)
}

// sweep drops buckets that have been idle long enough to have refilled, so the
// map cannot grow without bound. Callers must hold the mutex.
func (l *limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.idle {
		return
	}
	for key, visitor := range l.buckets {
		if now.Sub(visitor.seen) >= l.idle {
			delete(l.buckets, key)
		}
	}
	l.lastSweep = now
}
