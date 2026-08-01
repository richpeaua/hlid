// Package e2e contains black-box integration tests that exercise the assembled Hlid server
// end-to-end. This file drives the Slice-2 OIDC auth spine: config -> server.NewWithContext ->
// router -> auth middleware -> proxy -> upstream, against a fake OIDC provider (real discovery,
// real JWKS fetch, real token exchange over HTTP/TLS, real ID-token verification). No internals
// are mocked.
package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/richpeaua/hlid/internal/config"
	"github.com/richpeaua/hlid/internal/server"
)

const testKeyID = "auth-e2e-key-1"

// idTokenSpec configures the ID token the fake provider's token endpoint mints.
type idTokenSpec struct {
	subject  string
	email    string
	nonce    string
	audience string
}

// fakeOIDCProvider is a black-box fake OIDC discovery + JWKS + token endpoint, served over TLS
// (config.Provider.Issuer must be an absolute https URL) so tests never touch a real provider.
type fakeOIDCProvider struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	issuer string

	// nextToken configures the ID token minted by the next /token response.
	nextToken idTokenSpec
}

// newFakeOIDCProvider starts a TLS-backed fake provider serving discovery, JWKS, and token
// endpoints.
func newFakeOIDCProvider(t *testing.T) *fakeOIDCProvider {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	fp := &fakeOIDCProvider{key: key}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                fp.issuer,
			"authorization_endpoint":                fp.issuer + "/authorize",
			"token_endpoint":                        fp.issuer + "/token",
			"jwks_uri":                              fp.issuer + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &fp.key.PublicKey, KeyID: testKeyID, Algorithm: "RS256", Use: "sig"},
		}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		raw, err := fp.signIDToken(fp.nextToken)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fake-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     raw,
		})
	})

	fp.server = httptest.NewTLSServer(mux)
	t.Cleanup(fp.server.Close)
	fp.issuer = fp.server.URL
	return fp
}

// signIDToken mints a signed JWT for spec using fp's RSA key (published in /jwks).
func (fp *fakeOIDCProvider) signIDToken(spec idTokenSpec) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: fp.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", testKeyID),
	)
	if err != nil {
		return "", fmt.Errorf("new signer: %w", err)
	}

	stdClaims := jwt.Claims{
		Issuer:   fp.issuer,
		Subject:  spec.subject,
		Audience: jwt.Audience{spec.audience},
		Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt: jwt.NewNumericDate(time.Now()),
	}
	extra := struct {
		Email string `json:"email,omitempty"`
		Nonce string `json:"nonce,omitempty"`
	}{Email: spec.email, Nonce: spec.nonce}

	return jwt.Signed(signer).Claims(stdClaims).Claims(extra).Serialize()
}

// newAuthUpstream returns an httptest.Server standing in for the protected app: it echoes the
// request method, path, and body verbatim, and counts how many requests it has served, so tests
// can assert a denied request never reaches it.
func newAuthUpstream(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s:%s:%s", r.Method, r.URL.Path, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// authTestStack bundles the assembled Hlid server (over the auth-enforced config), the fake
// provider, and the test upstream, wired together so a client can drive the whole flow.
type authTestStack struct {
	hlid         *httptest.Server
	provider     *fakeOIDCProvider
	upstream     *httptest.Server
	upstreamHits *atomic.Int64
	clientID     string
	client       *http.Client // never follows redirects, so tests can inspect each hop
}

// newAuthTestStack builds the full auth-enforced Hlid server: real server.New(WithContext)
// handler, a fake OIDC provider (TLS discovery/JWKS/token endpoints), and a test upstream mounted
// at "/app/". ctx supplied to discovery (and to every request Hlid serves, via BaseContext)
// carries an *http.Client that trusts the fake provider's TLS certificate, so the whole flow
// (discovery, JWKS fetch, token exchange) runs without touching the real network.
func newAuthTestStack(t *testing.T) *authTestStack {
	t.Helper()

	fp := newFakeOIDCProvider(t)
	upstream, hits := newAuthUpstream(t)

	const secretEnv = "TEST_HLID_AUTH_E2E_CLIENT_SECRET"
	t.Setenv(secretEnv, "s3cr3t")

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate session key: %v", err)
	}
	t.Setenv("HLID_SESSION_KEY", base64.StdEncoding.EncodeToString(key))

	hlid := httptest.NewUnstartedServer(nil)
	redirectURL := "http://" + hlid.Listener.Addr().String() + "/auth/callback"

	cfg := &config.Config{
		Listen: ":0",
		Routes: []config.Route{
			{Path: "/app/", Upstream: upstream.URL},
		},
		Session: &config.Session{
			CookieName: "hlid_session",
			KeyFile:    "unused", // HLID_SESSION_KEY overrides
			TTL:        time.Hour,
		},
		Provider: &config.Provider{
			Issuer:          fp.issuer,
			ClientID:        "test-client",
			ClientSecretEnv: secretEnv,
			RedirectURL:     redirectURL,
			Scopes:          []string{"openid", "email"},
		},
	}

	// discoveryCtx carries the fake provider's TLS-trusting client both for OIDC discovery
	// (server.NewWithContext) and for every subsequent request Hlid serves (BaseContext below),
	// since the auth callback's token exchange reads its HTTP client from the request context.
	discoveryCtx := oidc.ClientContext(context.Background(), fp.server.Client())

	httpSrv, err := server.NewWithContext(discoveryCtx, cfg)
	if err != nil {
		t.Fatalf("server.NewWithContext: %v", err)
	}

	hlid.Config.Handler = httpSrv.Handler
	hlid.Config.BaseContext = func(net.Listener) context.Context { return discoveryCtx }
	hlid.Start()
	t.Cleanup(hlid.Close)

	return &authTestStack{
		hlid:         hlid,
		provider:     fp,
		upstream:     upstream,
		upstreamHits: hits,
		clientID:     cfg.Provider.ClientID,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// cookieNamed returns the cookie named name set on resp, if any.
func cookieNamed(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestE2E_UnauthenticatedRedirectsToLogin proves A1: an unauthenticated navigation request to a
// protected route is redirected (302) into the login flow (at the fake provider's authorization
// endpoint), and /healthz stays open (200 "ok") without any session.
func TestE2E_UnauthenticatedRedirectsToLogin(t *testing.T) {
	stack := newAuthTestStack(t)

	req, err := http.NewRequest(http.MethodGet, stack.hlid.URL+"/app/protected", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Sec-Fetch-Mode", "navigate")

	resp, err := stack.client.Do(req)
	if err != nil {
		t.Fatalf("GET /app/protected: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	providerURL, err := url.Parse(stack.provider.issuer)
	if err != nil {
		t.Fatalf("parse provider issuer: %v", err)
	}
	if loc.Host != providerURL.Host {
		t.Errorf("Location host = %q, want the fake provider's host %q", loc.Host, providerURL.Host)
	}
	if got := loc.Query().Get("client_id"); got != stack.clientID {
		t.Errorf("Location client_id = %q, want %q", got, stack.clientID)
	}
	if stack.upstreamHits.Load() != 0 {
		t.Errorf("upstream hits = %d, want 0 (unauthenticated request must never reach it)", stack.upstreamHits.Load())
	}

	healthzResp, err := stack.client.Get(stack.hlid.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer healthzResp.Body.Close()
	body, err := io.ReadAll(healthzResp.Body)
	if err != nil {
		t.Fatalf("read /healthz body: %v", err)
	}
	if healthzResp.StatusCode != http.StatusOK {
		t.Errorf("/healthz status = %d, want %d", healthzResp.StatusCode, http.StatusOK)
	}
	if string(body) != "ok" {
		t.Errorf("/healthz body = %q, want %q", body, "ok")
	}
}

// TestE2E_FullAuthCodeFlowReachesUpstream proves A2: driving the full authorization-code flow
// against the fake OIDC provider yields a session cookie, after which replaying the original
// request with that cookie reaches the test upstream with its body intact.
func TestE2E_FullAuthCodeFlowReachesUpstream(t *testing.T) {
	stack := newAuthTestStack(t)

	const originalBody = "original request body"

	// 1. Unauthenticated protected request -> 302 into the login flow, with a pre-auth cookie
	// binding state+nonce.
	loginReq, err := http.NewRequest(http.MethodPost, stack.hlid.URL+"/app/protected", strings.NewReader(originalBody))
	if err != nil {
		t.Fatalf("build login request: %v", err)
	}
	loginReq.Header.Set("Sec-Fetch-Mode", "navigate")

	loginResp, err := stack.client.Do(loginReq)
	if err != nil {
		t.Fatalf("POST /app/protected (unauthenticated): %v", err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", loginResp.StatusCode, http.StatusFound)
	}

	authURL, err := url.Parse(loginResp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	state := authURL.Query().Get("state")
	nonce := authURL.Query().Get("nonce")
	if state == "" || nonce == "" {
		t.Fatalf("expected state+nonce in Location, got %q", authURL)
	}
	preAuthCookie := cookieNamed(loginResp, "hlid_preauth")
	if preAuthCookie == nil {
		t.Fatal("no pre-auth cookie set on the login redirect")
	}

	// 2. Simulate the provider's redirect back to /auth/callback with a code; the fake provider's
	// token endpoint mints an ID token carrying the nonce it was given.
	stack.provider.nextToken = idTokenSpec{
		subject:  "user-42",
		email:    "user-42@example.com",
		nonce:    nonce,
		audience: stack.clientID,
	}

	cbURL := stack.hlid.URL + "/auth/callback?code=test-code&state=" + url.QueryEscape(state)
	cbReq, err := http.NewRequest(http.MethodGet, cbURL, nil)
	if err != nil {
		t.Fatalf("build callback request: %v", err)
	}
	cbReq.AddCookie(preAuthCookie)

	cbResp, err := stack.client.Do(cbReq)
	if err != nil {
		t.Fatalf("GET /auth/callback: %v", err)
	}
	defer cbResp.Body.Close()
	if cbResp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(cbResp.Body)
		t.Fatalf("callback status = %d, want %d, body=%s", cbResp.StatusCode, http.StatusFound, body)
	}

	sessionCookie := cookieNamed(cbResp, "hlid_session")
	if sessionCookie == nil {
		t.Fatal("no session cookie set by the callback")
	}

	// 3. Replay the ORIGINAL request, now with the session cookie: it must reach the upstream
	// with its body intact.
	replayReq, err := http.NewRequest(http.MethodPost, stack.hlid.URL+"/app/protected", strings.NewReader(originalBody))
	if err != nil {
		t.Fatalf("build replay request: %v", err)
	}
	replayReq.AddCookie(sessionCookie)

	replayResp, err := stack.client.Do(replayReq)
	if err != nil {
		t.Fatalf("POST /app/protected (authenticated): %v", err)
	}
	defer replayResp.Body.Close()
	replayBody, err := io.ReadAll(replayResp.Body)
	if err != nil {
		t.Fatalf("read replay body: %v", err)
	}
	if replayResp.StatusCode != http.StatusOK {
		t.Fatalf("replay status = %d, want %d, body=%s", replayResp.StatusCode, http.StatusOK, replayBody)
	}
	want := fmt.Sprintf("%s:%s:%s", http.MethodPost, "/app/protected", originalBody)
	if string(replayBody) != want {
		t.Errorf("upstream saw = %q, want %q", replayBody, want)
	}
	if stack.upstreamHits.Load() != 1 {
		t.Errorf("upstream hits = %d, want 1", stack.upstreamHits.Load())
	}
}

// TestE2E_DeniedSessionNeverReachesUpstream proves A3: a request with a missing or tampered
// session cookie is denied and never reaches the upstream.
func TestE2E_DeniedSessionNeverReachesUpstream(t *testing.T) {
	cases := []struct {
		name   string
		cookie *http.Cookie
	}{
		{name: "missing cookie", cookie: nil},
		{name: "tampered cookie", cookie: &http.Cookie{Name: "hlid_session", Value: "not-a-valid-sealed-session-value"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stack := newAuthTestStack(t)

			req, err := http.NewRequest(http.MethodGet, stack.hlid.URL+"/app/protected", nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			// Non-navigation, so the middleware fails closed with 401 instead of redirecting.
			req.Header.Set("Accept", "application/json")
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}

			resp, err := stack.client.Do(req)
			if err != nil {
				t.Fatalf("GET /app/protected: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
			}
			if stack.upstreamHits.Load() != 0 {
				t.Errorf("upstream hits = %d, want 0 (denied request must never reach it)", stack.upstreamHits.Load())
			}
		})
	}
}

// TestE2E_AuthMainBuildsAndFailsFastOnInvalidConfig proves A4: `go build ./cmd/hlid` succeeds and
// the resulting binary exits non-zero, via log.Fatal, both on a missing config file and on an
// auth-enabled config whose OIDC discovery is unreachable at startup.
func TestE2E_AuthMainBuildsAndFailsFastOnInvalidConfig(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "hlid")

	build := exec.Command("go", "build", "-o", binPath, "./cmd/hlid")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/hlid: %v\n%s", err, out)
	}

	runAndExpectFailure := func(t *testing.T, cfgPath string) string {
		t.Helper()
		run := exec.Command(binPath, "-config", cfgPath)
		out, err := run.CombinedOutput()
		if err == nil {
			t.Fatalf("expected non-zero exit, got success; output:\n%s", out)
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
		}
		if exitErr.ExitCode() == 0 {
			t.Fatalf("exit code = 0, want non-zero")
		}
		return string(out)
	}

	t.Run("missing config file", func(t *testing.T) {
		out := runAndExpectFailure(t, filepath.Join(t.TempDir(), "does-not-exist.yaml"))
		if !strings.Contains(out, "config") {
			t.Errorf("output = %q, want it to mention the config failure", out)
		}
	})

	t.Run("unreachable OIDC discovery", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "hlid.yaml")
		body := "" +
			"listen: \":0\"\n" +
			"routes:\n" +
			"  - path: \"/app/\"\n" +
			"    upstream: \"http://127.0.0.1:1\"\n" +
			"session:\n" +
			"  cookie_name: \"hlid_session\"\n" +
			"  key_file: \"unused\"\n" +
			"  ttl: 1h\n" +
			"provider:\n" +
			"  issuer: \"https://127.0.0.1:1\"\n" +
			"  client_id: \"test-client\"\n" +
			"  client_secret_env: \"TEST_HLID_AUTH_E2E_MAIN_SECRET\"\n" +
			"  redirect_url: \"https://app.example.com/auth/callback\"\n"
		if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}

		run := exec.Command(binPath, "-config", cfgPath)
		run.Env = append(run.Env,
			"HLID_SESSION_KEY="+base64.StdEncoding.EncodeToString(make([]byte, 32)),
			"TEST_HLID_AUTH_E2E_MAIN_SECRET=s3cr3t",
		)
		out, err := run.CombinedOutput()
		if err == nil {
			t.Fatalf("expected non-zero exit for unreachable OIDC discovery, got success; output:\n%s", out)
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
		}
		if exitErr.ExitCode() == 0 {
			t.Fatalf("exit code = 0, want non-zero")
		}
	})
}
