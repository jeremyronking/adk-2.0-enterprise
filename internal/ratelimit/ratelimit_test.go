package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestLimiter(perMin, perDay int) *Limiter {
	return &Limiter{
		visitors: make(map[string]*visitor),
		cfg:      Config{RunsPerMinute: perMin, RunsPerDay: perDay, Paths: []string{"/api/run"}},
		now:      time.Now,
	}
}

func request(path, ip string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, path, nil)
	r.Header.Set("X-Forwarded-For", ip)
	return r
}

func TestAllowsUpToDailyCapThenBlocks(t *testing.T) {
	l := newTestLimiter(60, 3)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 1; i <= 3; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, request("/api/run_sse", "203.0.113.7"))
		if rec.Code != http.StatusOK {
			t.Fatalf("run %d: got %d, want 200", i, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request("/api/run_sse", "203.0.113.7"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("4th run: got %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Error("expected Retry-After header on 429")
	}
}

func TestLimitIsPerIP(t *testing.T) {
	l := newTestLimiter(60, 1)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request("/api/run_sse", "198.51.100.1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("first IP: got %d, want 200", rec.Code)
	}

	// Same IP is now exhausted.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, request("/api/run_sse", "198.51.100.1"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("same IP again: got %d, want 429", rec.Code)
	}

	// A different visitor is unaffected.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, request("/api/run_sse", "198.51.100.2"))
	if rec.Code != http.StatusOK {
		t.Fatalf("second IP: got %d, want 200", rec.Code)
	}
}

func TestUnmeteredPathsPassThrough(t *testing.T) {
	l := newTestLimiter(60, 1)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Burn the daily allowance on the metered path.
	h.ServeHTTP(httptest.NewRecorder(), request("/api/run_sse", "203.0.113.9"))

	// The UI itself and session bookkeeping must still work.
	for _, path := range []string{"/", "/api/apps/chat_agent/users/demo/sessions", "/api/list-apps"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, request(path, "203.0.113.9"))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: got %d, want 200 (should not be metered)", path, rec.Code)
		}
	}
}

func TestDailyWindowRollsOver(t *testing.T) {
	l := newTestLimiter(60, 1)
	clock := time.Date(2026, 8, 1, 23, 59, 0, 0, time.UTC)
	l.now = func() time.Time { return clock }

	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	h.ServeHTTP(httptest.NewRecorder(), request("/api/run_sse", "203.0.113.5"))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request("/api/run_sse", "203.0.113.5"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("exhausted same day: got %d, want 429", rec.Code)
	}

	// Cross UTC midnight.
	clock = clock.Add(2 * time.Minute)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, request("/api/run_sse", "203.0.113.5"))
	if rec.Code != http.StatusOK {
		t.Fatalf("after UTC rollover: got %d, want 200", rec.Code)
	}
}

func TestBurstLimitSeparateFromDailyCap(t *testing.T) {
	// Generous daily cap, but only 1/min of burst.
	l := newTestLimiter(1, 100)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request("/api/run_sse", "203.0.113.11"))
	if rec.Code != http.StatusOK {
		t.Fatalf("first: got %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, request("/api/run_sse", "203.0.113.11"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("immediate second: got %d, want 429 from burst limit", rec.Code)
	}
}

func TestStatusDoesNotConsumeQuota(t *testing.T) {
	l := newTestLimiter(60, 3)
	r := request("/api/limits", "203.0.113.20")

	for i := 0; i < 5; i++ {
		if got := l.Status(r); got.Remaining != 3 {
			t.Fatalf("Status call %d consumed quota: remaining=%d, want 3", i, got.Remaining)
		}
	}
}

func TestClientIPPrefersLeftmostForwardedFor(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.1, 70.41.3.18, 150.172.238.178")
	if got := ClientIP(r); got != "203.0.113.1" {
		t.Errorf("ClientIP = %q, want the original client 203.0.113.1", got)
	}

	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.44:51234"
	if got := ClientIP(r); got != "192.0.2.44" {
		t.Errorf("ClientIP fallback = %q, want 192.0.2.44", got)
	}
}
