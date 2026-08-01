// Package ratelimit provides per-IP rate limiting for the public demo
// deployment. Agent runs invoke Gemini and cost real money, so the expensive
// endpoints get a hard daily cap in addition to a per-minute burst limit.
//
// Two tiers, matching the convention used across the jrk-ai-labs services:
//   - per-minute token bucket, to blunt bursts
//   - per-day counter, to cap sustained usage by a single visitor
package ratelimit

import (
	"crypto/subtle"
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
	// TotalRunsPerDay caps runs across ALL visitors combined. The per-IP limit
	// bounds what one visitor can spend; only this bounds the aggregate bill,
	// since nothing stops many distinct visitors each using their full quota.
	// Zero disables the global cap.
	TotalRunsPerDay int
	// Paths are the URL path prefixes treated as expensive (agent invocations).
	Paths []string
	// TrustCloudflare makes client IP resolution read CF-Connecting-IP. Enable
	// it only when Cloudflare proxies every request, and pair it with ingress
	// restrictions so the origin cannot be reached directly — otherwise a caller
	// can forge the header and mint unlimited quota buckets.
	TrustCloudflare bool
	// OriginSecret, when non-empty, requires metered requests to present it in
	// the X-Origin-Auth header. A Cloudflare Transform Rule injects the value,
	// so requests that skip Cloudflare and hit the origin URL directly are
	// rejected. This is what makes TrustCloudflare safe: without it, anyone who
	// finds the origin can forge CF-Connecting-IP and mint unlimited quota.
	//
	// Only metered paths are gated. The UI still loads from the bare origin —
	// a misconfigured rule degrades the demo rather than taking the site down.
	OriginSecret string
}

// OriginAuthHeader carries the shared secret injected by the edge proxy.
const OriginAuthHeader = "X-Origin-Auth"

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
	// Aggregate counter across every visitor, rolled over with the UTC day.
	totalCount int
	totalDay   time.Time
	// now is overridable so tests can advance the clock.
	now func() time.Time
}

// rollGlobalWindow resets the aggregate counter when the UTC day changes.
// Callers must hold l.mu.
func (l *Limiter) rollGlobalWindow() {
	if d := dayBoundary(l.now()); d.After(l.totalDay) {
		l.totalDay = d
		l.totalCount = 0
	}
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
	// SharedRemaining is what is left of the aggregate daily budget across all
	// visitors. A caller can have per-IP allowance left while this is zero.
	SharedRemaining int `json:"sharedRemaining"`
	SharedPerDay    int `json:"sharedPerDay"`
}

// Status returns the caller's current daily usage.
func (l *Limiter) Status(r *http.Request) Status {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.rollGlobalWindow()
	v := l.getVisitor(ClientIP(r, l.cfg.TrustCloudflare))

	remaining := max0(l.cfg.RunsPerDay - v.dayCount)
	shared := max0(l.cfg.TotalRunsPerDay - l.totalCount)

	return Status{
		RunsPerDay:      l.cfg.RunsPerDay,
		Used:            v.dayCount,
		Remaining:       remaining,
		SharedRemaining: shared,
		SharedPerDay:    l.cfg.TotalRunsPerDay,
	}
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
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

		// Reject metered requests that did not come through the edge proxy.
		// Checked before any quota accounting so a direct caller cannot even
		// probe the counters.
		if l.cfg.OriginSecret != "" &&
			subtle.ConstantTimeCompare([]byte(r.Header.Get(OriginAuthHeader)), []byte(l.cfg.OriginSecret)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":   "DIRECT_ORIGIN_ACCESS",
				"message": "Live runs must be requested through the public demo URL.",
			})
			return
		}

		l.mu.Lock()
		l.rollGlobalWindow()
		v := l.getVisitor(ClientIP(r, l.cfg.TrustCloudflare))

		// Aggregate cap first: once the day's budget is gone it is gone for
		// everyone, regardless of an individual visitor's remaining allowance.
		if l.cfg.TotalRunsPerDay > 0 && l.totalCount >= l.cfg.TotalRunsPerDay {
			resetIn := int(time.Until(l.totalDay.Add(24 * time.Hour)).Seconds())
			l.mu.Unlock()
			writeLimitResponse(w, http.StatusTooManyRequests, resetIn,
				"DAILY_BUDGET_REACHED",
				"The shared daily budget for live agent runs is spent. It resets at 00:00 UTC. The simulated pipeline is unlimited and shows the same workflow.")
			return
		}

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
		l.totalCount++
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

// ClientIP resolves the caller's address for quota accounting.
//
// X-Forwarded-For is client-supplied data that proxies APPEND to. Trusting the
// leftmost entry lets any caller mint a fresh quota bucket by sending a made-up
// value, so we always read from the right — the entries a proxy appended — and
// never from the left.
//
// trustCloudflare selects which hop to believe:
//
//   - true:  CF-Connecting-IP, which Cloudflare overwrites on every request and
//     a client therefore cannot forge. Only correct when Cloudflare is
//     guaranteed to be in front; otherwise a direct caller could forge it.
//   - false: the rightmost X-Forwarded-For entry, appended by Google's front end
//     for a request that reached Cloud Run directly.
func ClientIP(r *http.Request, trustCloudflare bool) string {
	if trustCloudflare {
		if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
			return ip
		}
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.LastIndexByte(xff, ','); i >= 0 {
			if ip := strings.TrimSpace(xff[i+1:]); ip != "" {
				return ip
			}
		} else if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
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
