// Package ratelimit provides per-IP rate limiting for the public demo
// deployment. Agent runs invoke Gemini and cost real money, so the expensive
// endpoints get a hard daily cap in addition to a per-minute burst limit.
//
// Two tiers, matching the convention used across the jrk-ai-labs services:
//   - per-minute token bucket, to blunt bursts
//   - per-day counter, to cap sustained usage by a single visitor
package ratelimit

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Config holds the tunable limits. Zero values disable the corresponding tier.
type Config struct {
	// RunsPerMinute is the burst allowance for expensive endpoints.
	RunsPerMinute int
	// RunsPerDay is the hard daily cap for expensive endpoints, per IP.
	RunsPerDay int
	// Paths are the URL path prefixes treated as expensive (agent invocations).
	Paths []string
}

type visitor struct {
	burst    *rate.Limiter
	dayCount int
	dayStart time.Time
	lastSeen time.Time
}

// Limiter tracks per-IP usage. The zero value is not usable; call New.
type Limiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	cfg      Config
	// now is overridable so tests can advance the clock.
	now func() time.Time
}

// New creates a Limiter and starts a background sweep for stale visitors.
func New(cfg Config) *Limiter {
	l := &Limiter{
		visitors: make(map[string]*visitor),
		cfg:      cfg,
		now:      time.Now,
	}
	go l.sweepStale()
	return l
}

// dayBoundary returns the start of the UTC day containing t. Daily quotas reset
// at UTC midnight so the cap is predictable regardless of visitor timezone.
func dayBoundary(t time.Time) time.Time {
	return t.UTC().Truncate(24 * time.Hour)
}

func (l *Limiter) getVisitor(ip string) *visitor {
	now := l.now()
	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{
			burst:    rate.NewLimiter(rate.Limit(float64(l.cfg.RunsPerMinute)/60.0), l.cfg.RunsPerMinute),
			dayStart: dayBoundary(now),
		}
		l.visitors[ip] = v
	}
	// Roll the daily window over when the UTC day changes.
	if d := dayBoundary(now); d.After(v.dayStart) {
		v.dayStart = d
		v.dayCount = 0
	}
	v.lastSeen = now
	return v
}

func (l *Limiter) sweepStale() {
	for {
		time.Sleep(10 * time.Minute)
		l.mu.Lock()
		for ip, v := range l.visitors {
			// Keep entries for a full day so the daily cap survives idle gaps.
			if l.now().Sub(v.lastSeen) > 25*time.Hour {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

func (l *Limiter) isExpensive(path string) bool {
	for _, p := range l.cfg.Paths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// Status reports the remaining daily allowance for an IP without consuming it.
type Status struct {
	RunsPerDay int `json:"runsPerDay"`
	Used       int `json:"used"`
	Remaining  int `json:"remaining"`
}

// Status returns the caller's current daily usage.
func (l *Limiter) Status(r *http.Request) Status {
	l.mu.Lock()
	defer l.mu.Unlock()

	v := l.getVisitor(ClientIP(r))
	remaining := l.cfg.RunsPerDay - v.dayCount
	if remaining < 0 {
		remaining = 0
	}
	return Status{RunsPerDay: l.cfg.RunsPerDay, Used: v.dayCount, Remaining: remaining}
}

// Middleware applies the limits to expensive paths and passes everything else
// through untouched. Static assets and session bookkeeping stay unmetered so the
// page still loads for a visitor who has used up their runs.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.isExpensive(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		l.mu.Lock()
		v := l.getVisitor(ClientIP(r))

		if l.cfg.RunsPerDay > 0 && v.dayCount >= l.cfg.RunsPerDay {
			resetIn := int(time.Until(v.dayStart.Add(24 * time.Hour)).Seconds())
			l.mu.Unlock()
			writeLimitResponse(w, http.StatusTooManyRequests, resetIn,
				"DAILY_LIMIT_REACHED",
				"You have used today's demo runs. The quota resets at 00:00 UTC. Switch Execution Mode to the simulated pipeline to keep exploring the UI.")
			return
		}

		if l.cfg.RunsPerMinute > 0 && !v.burst.Allow() {
			l.mu.Unlock()
			writeLimitResponse(w, http.StatusTooManyRequests, 60,
				"RATE_LIMIT_EXCEEDED",
				"Too many runs in a short window. Please wait a minute and try again.")
			return
		}

		v.dayCount++
		remaining := l.cfg.RunsPerDay - v.dayCount
		l.mu.Unlock()

		if remaining < 0 {
			remaining = 0
		}
		w.Header().Set("X-RateLimit-Limit-Day", itoa(l.cfg.RunsPerDay))
		w.Header().Set("X-RateLimit-Remaining-Day", itoa(remaining))
		next.ServeHTTP(w, r)
	})
}

// StatusHandler exposes the caller's remaining allowance so the UI can show it
// before a run is attempted.
func (l *Limiter) StatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(l.Status(r))
}

// ClientIP resolves the caller's address. Behind Cloud Run the direct peer is
// the load balancer, so the leftmost X-Forwarded-For entry is the real client.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func writeLimitResponse(w http.ResponseWriter, code, retryAfter int, errCode, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", itoa(retryAfter))
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":             errCode,
		"message":           msg,
		"retryAfterSeconds": retryAfter,
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
