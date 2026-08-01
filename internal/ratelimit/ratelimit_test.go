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

func TestClientIPReadsRightmostForwardedFor(t *testing.T) {
	// Proxies append, so the rightmost entry is the one our trusted hop added.
	// Everything to its left is caller-supplied and must not be believed.
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.1, 70.41.3.18, 150.172.238.178")
	if got := ClientIP(r, false); got != "150.172.238.178" {
		t.Errorf("ClientIP = %q, want the proxy-appended 150.172.238.178", got)
	}

	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.44:51234"
	if got := ClientIP(r, false); got != "192.0.2.44" {
		t.Errorf("ClientIP fallback = %q, want 192.0.2.44", got)
	}
}

// A caller must not be able to mint fresh quota buckets by inventing
// X-Forwarded-For values. This is the bypass the leftmost-entry version had.
func TestSpoofedForwardedForCannotResetQuota(t *testing.T) {
	l := newTestLimiter(60, 2)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// One real client (the proxy always appends 198.51.100.77) rotating
	// through invented leftmost values on every request.
	attempt := func(spoof string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/run_sse", nil)
		r.Header.Set("X-Forwarded-For", spoof+", 198.51.100.77")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}

	if got := attempt("1.1.1.1"); got != http.StatusOK {
		t.Fatalf("run 1: got %d, want 200", got)
	}
	if got := attempt("2.2.2.2"); got != http.StatusOK {
		t.Fatalf("run 2: got %d, want 200", got)
	}
	if got := attempt("3.3.3.3"); got != http.StatusTooManyRequests {
		t.Fatalf("run 3 with a new spoofed IP: got %d, want 429 — quota was bypassed", got)
	}
}

func TestOriginSecretGatesMeteredPaths(t *testing.T) {
	l := newTestLimiter(60, 5)
	l.cfg.OriginSecret = "s3cr3t"
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No header at all — a direct hit on the origin URL.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request("/api/run_sse", "203.0.113.30"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("missing secret: got %d, want 403", rec.Code)
	}

	// Wrong value.
	r := request("/api/run_sse", "203.0.113.30")
	r.Header.Set(OriginAuthHeader, "guess")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Errorf("wrong secret: got %d, want 403", rec.Code)
	}

	// Correct value, as injected by the edge.
	r = request("/api/run_sse", "203.0.113.30")
	r.Header.Set(OriginAuthHeader, "s3cr3t")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("correct secret: got %d, want 200", rec.Code)
	}
}

func TestOriginSecretRejectionDoesNotConsumeQuota(t *testing.T) {
	l := newTestLimiter(60, 1)
	l.cfg.OriginSecret = "s3cr3t"
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Hammer the origin directly; none of it should touch the counters.
	for i := 0; i < 10; i++ {
		h.ServeHTTP(httptest.NewRecorder(), request("/api/run_sse", "203.0.113.31"))
	}

	r := request("/api/run_sse", "203.0.113.31")
	r.Header.Set(OriginAuthHeader, "s3cr3t")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("legitimate request after rejected ones: got %d, want 200", rec.Code)
	}
}

func TestUIStillLoadsFromBareOrigin(t *testing.T) {
	l := newTestLimiter(60, 3)
	l.cfg.OriginSecret = "s3cr3t"
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// A broken Transform Rule should degrade the demo, not take the site down.
	for _, path := range []string{"/", "/api/limits", "/api/list-apps"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, request(path, "203.0.113.32"))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: got %d, want 200 (must not require the origin secret)", path, rec.Code)
		}
	}
}

func TestCloudflareConnectingIPPreferredWhenTrusted(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("CF-Connecting-IP", "203.0.113.50")
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 9.9.9.9")

	if got := ClientIP(r, true); got != "203.0.113.50" {
		t.Errorf("trusted: ClientIP = %q, want CF-Connecting-IP 203.0.113.50", got)
	}
	// Untrusted deployments must ignore the forgeable Cloudflare header.
	if got := ClientIP(r, false); got != "9.9.9.9" {
		t.Errorf("untrusted: ClientIP = %q, want rightmost XFF 9.9.9.9", got)
	}
}

func TestSpoofedCloudflareHeaderIgnoredWhenNotTrusted(t *testing.T) {
	l := newTestLimiter(60, 1)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	attempt := func(spoof string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/run_sse", nil)
		r.Header.Set("CF-Connecting-IP", spoof)
		r.Header.Set("X-Forwarded-For", "203.0.113.90")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}

	if got := attempt("8.8.8.8"); got != http.StatusOK {
		t.Fatalf("first: got %d, want 200", got)
	}
	if got := attempt("4.4.4.4"); got != http.StatusTooManyRequests {
		t.Fatalf("second with new forged CF header: got %d, want 429", got)
	}
}
