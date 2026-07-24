package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/richpeaua/hlid/internal/config"
)

func validConfig(upstream string) *config.Config {
	return &config.Config{
		Listen: ":8443",
		Routes: []config.Route{
			{Path: "/app/", Upstream: upstream},
		},
	}
}

// A1: GET /healthz -> 200 "ok", and is not routed to an upstream.
func TestHealthzNotProxied(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg := validConfig(upstream.URL)
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	buf := make([]byte, 2)
	n, _ := resp.Body.Read(buf)
	if string(buf[:n]) != "ok" {
		t.Fatalf("body = %q, want %q", string(buf[:n]), "ok")
	}

	if upstreamHit {
		t.Fatal("expected /healthz to not be proxied to upstream")
	}
}

// A2: a non-health path is dispatched through the router to the matching upstream.
func TestNonHealthPathDispatchedToRouter(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg := validConfig(upstream.URL)
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/app/foo")
	if err != nil {
		t.Fatalf("GET /app/foo: %v", err)
	}
	defer resp.Body.Close()

	if !upstreamHit {
		t.Fatal("expected /app/foo to be dispatched to the upstream via the router")
	}
}

// A3: a panicking test handler is recovered as 500. Unit-tests the recoverPanic middleware
// directly (it is applied to the whole mux inside New's handler chain).
func TestPanicRecovered(t *testing.T) {
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)

	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("panic escaped recoverPanic: %v", p)
			}
		}()
		recoverPanic(panicking).ServeHTTP(rec, req)
	}()

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// A4: New errors when cfg is invalid; Addr/timeouts are set on success.
func TestNewInvalidConfigErrors(t *testing.T) {
	cfg := &config.Config{} // empty: fails Validate (no Listen, no Routes)

	if _, err := New(cfg); err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
}

func TestNewSetsAddrAndTimeouts(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg := validConfig(upstream.URL)
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	if srv.Addr != cfg.Listen {
		t.Fatalf("Addr = %q, want %q", srv.Addr, cfg.Listen)
	}
	if srv.ReadHeaderTimeout <= 0 {
		t.Fatalf("ReadHeaderTimeout = %v, want > 0", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout <= 0 {
		t.Fatalf("IdleTimeout = %v, want > 0", srv.IdleTimeout)
	}
	if srv.IdleTimeout < time.Second {
		t.Fatalf("IdleTimeout = %v, want a sane (>=1s) value", srv.IdleTimeout)
	}
}
