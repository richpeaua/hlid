package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

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

// fakeOIDCDiscovery is a minimal fake OIDC discovery + JWKS endpoint, so auth setup in these
// tests never touches the real network. It never needs to mint tokens: these tests only exercise
// unauthenticated requests (denied before Callback would run) and the auth-less path.
type fakeOIDCDiscovery struct {
	server *httptest.Server
	issuer string
}

func newFakeOIDCDiscovery(t *testing.T) *fakeOIDCDiscovery {
	t.Helper()
	fd := &fakeOIDCDiscovery{}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                fd.issuer,
			"authorization_endpoint":                fd.issuer + "/authorize",
			"token_endpoint":                        fd.issuer + "/token",
			"jwks_uri":                              fd.issuer + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
	})
	fd.server = httptest.NewTLSServer(mux) // https: config.Provider.validate requires an https issuer
	t.Cleanup(fd.server.Close)
	fd.issuer = fd.server.URL
	return fd
}

// authConfig builds a valid auth config (Session + Provider) wired to a fake, network-free OIDC
// discovery server and a session key set via env (t.Setenv so it's cleaned up automatically). It
// also returns the discovery context: OIDC discovery must be pointed at the fake server's client
// (which trusts its self-signed cert) rather than the real network.
func authConfig(t *testing.T, upstream string) (*config.Config, context.Context) {
	t.Helper()
	fd := newFakeOIDCDiscovery(t)
	ctx := oidc.ClientContext(context.Background(), fd.server.Client())

	const secretEnv = "TEST_HLID_SERVER_CLIENT_SECRET"
	t.Setenv(secretEnv, "s3cr3t")

	key := make([]byte, 32)
	t.Setenv("HLID_SESSION_KEY", base64.StdEncoding.EncodeToString(key))

	cfg := validConfig(upstream)
	cfg.Session = &config.Session{
		CookieName: "hlid_session",
		KeyFile:    "unused-because-env-set",
		TTL:        time.Hour,
	}
	cfg.Provider = &config.Provider{
		Issuer:          fd.issuer,
		ClientID:        "test-client",
		ClientSecretEnv: secretEnv,
		RedirectURL:     "https://app.example.com/auth/callback",
		Scopes:          []string{"openid"},
	}
	return cfg, ctx
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

// A5 (auth-config path): /healthz and /auth/login are reachable with no session cookie; an
// unauthenticated proxied request is denied (302/401) and never reaches the upstream.
func TestNewWithAuthDeniesUnauthenticatedProxiedRequest(t *testing.T) {
	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg, ctx := authConfig(t, upstream.URL)
	srv, err := NewWithContext(ctx, cfg)
	if err != nil {
		t.Fatalf("NewWithContext: unexpected error: %v", err)
	}

	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	t.Run("healthz reachable with no session", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/healthz")
		if err != nil {
			t.Fatalf("GET /healthz: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	t.Run("auth login reachable with no session", func(t *testing.T) {
		client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}
		resp, err := client.Get(ts.URL + "/auth/login")
		if err != nil {
			t.Fatalf("GET /auth/login: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			t.Fatalf("status = %d, want %d (Login redirects to the provider)", resp.StatusCode, http.StatusFound)
		}
	})

	t.Run("unauthenticated proxied request denied, never reaches upstream", func(t *testing.T) {
		client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/app/foo", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /app/foo: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusFound {
			t.Fatalf("status = %d, want 401 or 302", resp.StatusCode)
		}
		if upstreamHit {
			t.Fatal("expected unauthenticated request to never reach the upstream")
		}
	})
}

// A4 (Slice-1 auth-less shape, unchanged): covered by TestHealthzNotProxied,
// TestNonHealthPathDispatchedToRouter, TestPanicRecovered, TestNewInvalidConfigErrors, and
// TestNewSetsAddrAndTimeouts above, all run against validConfig (no Session/Provider).

// A6: New errors when auth setup fails (missing session key) and when exactly one of
// session/provider is set.
func TestNewAuthSetupErrors(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	t.Run("missing session key", func(t *testing.T) {
		cfg, ctx := authConfig(t, upstream.URL)
		t.Setenv("HLID_SESSION_KEY", "") // unset the key the fixture set
		cfg.Session.KeyFile = "/nonexistent/path/to/key"

		if _, err := NewWithContext(ctx, cfg); err == nil {
			t.Fatal("expected error for missing session key, got nil")
		}
	})

	t.Run("session set, provider unset", func(t *testing.T) {
		cfg := validConfig(upstream.URL)
		cfg.Session = &config.Session{CookieName: "hlid_session", KeyFile: "unused", TTL: time.Hour}

		if _, err := NewWithContext(context.Background(), cfg); err == nil {
			t.Fatal("expected error for half-set auth config (session only), got nil")
		}
	})

	t.Run("provider set, session unset", func(t *testing.T) {
		cfg := validConfig(upstream.URL)
		cfg.Provider = &config.Provider{
			Issuer:          "https://example.com",
			ClientID:        "c",
			ClientSecretEnv: "UNUSED",
			RedirectURL:     "https://app.example.com/auth/callback",
			Scopes:          []string{"openid"},
		}

		if _, err := NewWithContext(context.Background(), cfg); err == nil {
			t.Fatal("expected error for half-set auth config (provider only), got nil")
		}
	})
}

// A6 (deterministic expiry via injected clock): the Authenticator's clock is exercised via
// internal/auth's own tests (TestCallbackSuccess uses a fixed a.now); here we only verify that
// server.NewWithContext's auth wiring uses fakes/no real network (fakeOIDCDiscovery, no live
// issuer), consistent with that constraint.
func TestNewWithAuthUsesNoRealNetwork(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg, ctx := authConfig(t, upstream.URL)
	if _, err := NewWithContext(ctx, cfg); err != nil {
		t.Fatalf("NewWithContext against fake discovery server: unexpected error: %v", err)
	}
}
