package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/richpeaua/hlid/internal/config"
	"github.com/richpeaua/hlid/internal/session"
)

const testKeyID = "test-key-1"

// idTokenSpec configures the ID token the fake provider's token endpoint mints.
type idTokenSpec struct {
	subject     string
	email       string
	nonce       string
	audience    string
	expiry      time.Time
	wrongSigner bool // sign with a key not in the published JWKS, to simulate a bad token
	omit        bool // omit id_token from the token response entirely
}

// fakeProvider is a table-driven fake OIDC discovery + JWKS + token endpoint,
// so tests never touch the real network.
type fakeProvider struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	other  *rsa.PrivateKey // an unpublished key, for bad-signature tests
	issuer string

	// nextToken configures the next /token response; set by the test before
	// driving Callback.
	nextToken idTokenSpec
	// tokenFail, if true, makes the token endpoint return 400 (exchange failure).
	tokenFail bool
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key (other): %v", err)
	}

	fp := &fakeProvider{key: key, other: other}

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
		if fp.tokenFail {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
			return
		}

		resp := map[string]any{
			"access_token": "fake-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}
		if !fp.nextToken.omit {
			raw, err := fp.signIDToken(fp.nextToken)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			resp["id_token"] = raw
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	fp.server = httptest.NewServer(mux)
	t.Cleanup(fp.server.Close)
	fp.issuer = fp.server.URL
	return fp
}

// signIDToken mints a signed JWT for spec, using fp.key unless spec.wrongSigner
// selects fp.other (a key absent from the published JWKS).
func (fp *fakeProvider) signIDToken(spec idTokenSpec) (string, error) {
	signingKey := fp.key
	kid := testKeyID
	if spec.wrongSigner {
		signingKey = fp.other
		kid = "other-key"
	}

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: signingKey}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid))
	if err != nil {
		return "", fmt.Errorf("new signer: %w", err)
	}

	exp := spec.expiry
	if exp.IsZero() {
		exp = time.Now().Add(time.Hour)
	}

	stdClaims := jwt.Claims{
		Issuer:   fp.issuer,
		Subject:  spec.subject,
		Audience: jwt.Audience{spec.audience},
		Expiry:   jwt.NewNumericDate(exp),
		IssuedAt: jwt.NewNumericDate(time.Now()),
	}
	extra := struct {
		Email string `json:"email,omitempty"`
		Nonce string `json:"nonce,omitempty"`
	}{Email: spec.email, Nonce: spec.nonce}

	return jwt.Signed(signer).Claims(stdClaims).Claims(extra).Serialize()
}

// newTestAuthenticator builds an Authenticator against fp, with clientSecretEnv set.
func newTestAuthenticator(t *testing.T, fp *fakeProvider, sess config.Session) *Authenticator {
	t.Helper()

	const secretEnv = "TEST_HLID_CLIENT_SECRET"
	t.Setenv(secretEnv, "s3cr3t")

	prov := config.Provider{
		Issuer:          fp.issuer,
		ClientID:        "test-client",
		ClientSecretEnv: secretEnv,
		RedirectURL:     "https://app.example.com/auth/callback",
		Scopes:          []string{"openid", "email"},
	}

	store, err := session.NewStore(make([]byte, 32))
	if err != nil {
		t.Fatalf("session.NewStore: %v", err)
	}

	a, err := New(context.Background(), prov, sess, store)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

func testSession() config.Session {
	return config.Session{CookieName: "hlid_session", KeyFile: "unused", TTL: time.Hour}
}

// A1: New errors on a missing client secret env and on unreachable discovery;
// succeeds against a fake OIDC discovery server.
func TestNew(t *testing.T) {
	t.Run("missing client secret env", func(t *testing.T) {
		fp := newFakeProvider(t)
		prov := config.Provider{
			Issuer:          fp.issuer,
			ClientID:        "test-client",
			ClientSecretEnv: "TEST_HLID_UNSET_SECRET_ENV",
			RedirectURL:     "https://app.example.com/auth/callback",
			Scopes:          []string{"openid"},
		}
		store, err := session.NewStore(make([]byte, 32))
		if err != nil {
			t.Fatalf("session.NewStore: %v", err)
		}
		if _, err := New(context.Background(), prov, testSession(), store); err == nil {
			t.Fatal("New() = nil error, want error for missing client secret env")
		}
	})

	t.Run("unreachable discovery", func(t *testing.T) {
		t.Setenv("TEST_HLID_CLIENT_SECRET_UNREACHABLE", "s3cr3t")
		prov := config.Provider{
			Issuer:          "https://127.0.0.1:1", // nothing listens here
			ClientID:        "test-client",
			ClientSecretEnv: "TEST_HLID_CLIENT_SECRET_UNREACHABLE",
			RedirectURL:     "https://app.example.com/auth/callback",
			Scopes:          []string{"openid"},
		}
		store, err := session.NewStore(make([]byte, 32))
		if err != nil {
			t.Fatalf("session.NewStore: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if _, err := New(ctx, prov, testSession(), store); err == nil {
			t.Fatal("New() = nil error, want error for unreachable discovery")
		}
	})

	t.Run("succeeds against fake discovery server", func(t *testing.T) {
		fp := newFakeProvider(t)
		a := newTestAuthenticator(t, fp, testSession())
		if a == nil {
			t.Fatal("New() returned nil Authenticator")
		}
	})
}

// A2: Login sets the pre-auth cookie and returns 302 to an authorization URL
// carrying the expected client_id, redirect_uri, scope, and the cookie's
// state+nonce (parsed from Location).
func TestLogin(t *testing.T) {
	fp := newFakeProvider(t)
	a := newTestAuthenticator(t, fp, testSession())

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	w := httptest.NewRecorder()

	a.Login(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusFound)
	}

	var preAuthCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == preAuthCookieName {
			preAuthCookie = c
		}
	}
	if preAuthCookie == nil {
		t.Fatal("no pre-auth cookie set")
	}
	if !preAuthCookie.HttpOnly || !preAuthCookie.Secure {
		t.Fatalf("pre-auth cookie not httponly+secure: %+v", preAuthCookie)
	}

	st, ok := a.readPreAuthCookie(&http.Request{Header: http.Header{"Cookie": {preAuthCookie.Name + "=" + preAuthCookie.Value}}})
	if !ok {
		t.Fatal("could not decode pre-auth cookie set by Login")
	}
	if st.State == "" || st.Nonce == "" {
		t.Fatalf("pre-auth state incomplete: %+v", st)
	}

	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	q := loc.Query()
	if got := q.Get("client_id"); got != "test-client" {
		t.Errorf("client_id = %q, want %q", got, "test-client")
	}
	if got := q.Get("redirect_uri"); got != "https://app.example.com/auth/callback" {
		t.Errorf("redirect_uri = %q, want %q", got, "https://app.example.com/auth/callback")
	}
	if got := q.Get("scope"); got != "openid email" {
		t.Errorf("scope = %q, want %q", got, "openid email")
	}
	if got := q.Get("state"); got != st.State {
		t.Errorf("state = %q, want cookie state %q", got, st.State)
	}
	if got := q.Get("nonce"); got != st.Nonce {
		t.Errorf("nonce = %q, want cookie nonce %q", got, st.Nonce)
	}
}

// callbackRequest builds a GET /auth/callback request carrying the given
// query params and pre-auth cookie (if any).
func callbackRequest(t *testing.T, query url.Values, preAuth *http.Cookie) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?"+query.Encode(), nil)
	if preAuth != nil {
		req.AddCookie(preAuth)
	}
	return req
}

// sealPreAuthCookie builds the pre-auth cookie Callback expects, bypassing Login
// so the test controls state/nonce/path directly.
func sealPreAuthCookie(t *testing.T, a *Authenticator, st preAuthState) *http.Cookie {
	t.Helper()
	w := httptest.NewRecorder()
	if err := a.setPreAuthCookie(w, st); err != nil {
		t.Fatalf("setPreAuthCookie: %v", err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	return cookies[0]
}

// A3 + A5: Callback with a valid code + matching state + valid ID token (nonce
// match) sets a session cookie that session.Store.Open decrypts to the
// expected Identity (with a deterministic, injected-clock Expiry), and 302s to
// the saved path.
func TestCallbackSuccess(t *testing.T) {
	fp := newFakeProvider(t)
	sess := testSession()
	a := newTestAuthenticator(t, fp, sess)

	fixedNow := time.Date(2030, 5, 6, 7, 8, 9, 0, time.UTC)
	a.now = func() time.Time { return fixedNow }

	pre := preAuthState{State: "the-state", Nonce: "the-nonce", Path: "/app/dashboard"}
	cookie := sealPreAuthCookie(t, a, pre)

	fp.nextToken = idTokenSpec{
		subject:  "user-42",
		email:    "user-42@example.com",
		nonce:    pre.Nonce,
		audience: "test-client",
	}

	req := callbackRequest(t, url.Values{"code": {"good-code"}, "state": {pre.State}}, cookie)
	w := httptest.NewRecorder()

	a.Callback(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusFound {
		body := w.Body.String()
		t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, http.StatusFound, body)
	}
	if got := resp.Header.Get("Location"); got != pre.Path {
		t.Errorf("Location = %q, want %q", got, pre.Path)
	}

	var sessionCookie *http.Cookie
	var clearedPreAuth bool
	for _, c := range resp.Cookies() {
		if c.Name == sess.CookieName {
			sessionCookie = c
		}
		if c.Name == preAuthCookieName && c.MaxAge < 0 {
			clearedPreAuth = true
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie set")
	}
	if !clearedPreAuth {
		t.Error("pre-auth cookie was not cleared")
	}

	store, err := session.NewStore(make([]byte, 32))
	if err != nil {
		t.Fatalf("session.NewStore: %v", err)
	}
	id, err := store.Open(sessionCookie.Value)
	if err != nil {
		t.Fatalf("store.Open(session cookie): %v", err)
	}
	if id.Subject != "user-42" || id.Email != "user-42@example.com" {
		t.Errorf("Identity = %+v, want sub=user-42 email=user-42@example.com", id)
	}
	wantExpiry := fixedNow.Add(sess.TTL)
	if !id.Expiry.Equal(wantExpiry) {
		t.Errorf("Identity.Expiry = %v, want %v (fixed clock + TTL)", id.Expiry, wantExpiry)
	}
}

// A4: Callback fails closed (no session cookie, non-2xx/302-to-upstream) on:
// missing pre-auth cookie, state mismatch, exchange failure, bad ID token,
// nonce mismatch.
func TestCallbackFailsClosed(t *testing.T) {
	newSetup := func(t *testing.T) (*Authenticator, *fakeProvider) {
		fp := newFakeProvider(t)
		a := newTestAuthenticator(t, fp, testSession())
		return a, fp
	}

	assertFailsClosed := func(t *testing.T, w *httptest.ResponseRecorder) {
		t.Helper()
		resp := w.Result()
		if resp.StatusCode < 400 || resp.StatusCode >= 500 {
			t.Fatalf("status = %d, want a 4xx (fail closed)", resp.StatusCode)
		}
		for _, c := range resp.Cookies() {
			if c.Name == testSession().CookieName && c.Value != "" {
				t.Fatalf("session cookie was set on failure path: %+v", c)
			}
		}
	}

	t.Run("missing pre-auth cookie", func(t *testing.T) {
		a, _ := newSetup(t)
		req := callbackRequest(t, url.Values{"code": {"c"}, "state": {"s"}}, nil)
		w := httptest.NewRecorder()
		a.Callback(w, req)
		assertFailsClosed(t, w)
	})

	t.Run("state mismatch", func(t *testing.T) {
		a, _ := newSetup(t)
		pre := preAuthState{State: "expected-state", Nonce: "n", Path: "/"}
		cookie := sealPreAuthCookie(t, a, pre)
		req := callbackRequest(t, url.Values{"code": {"c"}, "state": {"wrong-state"}}, cookie)
		w := httptest.NewRecorder()
		a.Callback(w, req)
		assertFailsClosed(t, w)
	})

	t.Run("exchange failure", func(t *testing.T) {
		a, fp := newSetup(t)
		fp.tokenFail = true
		pre := preAuthState{State: "s", Nonce: "n", Path: "/"}
		cookie := sealPreAuthCookie(t, a, pre)
		req := callbackRequest(t, url.Values{"code": {"c"}, "state": {pre.State}}, cookie)
		w := httptest.NewRecorder()
		a.Callback(w, req)
		assertFailsClosed(t, w)
	})

	t.Run("bad id token (signature)", func(t *testing.T) {
		a, fp := newSetup(t)
		pre := preAuthState{State: "s", Nonce: "n", Path: "/"}
		cookie := sealPreAuthCookie(t, a, pre)
		fp.nextToken = idTokenSpec{
			subject: "user-1", nonce: pre.Nonce, audience: "test-client", wrongSigner: true,
		}
		req := callbackRequest(t, url.Values{"code": {"c"}, "state": {pre.State}}, cookie)
		w := httptest.NewRecorder()
		a.Callback(w, req)
		assertFailsClosed(t, w)
	})

	t.Run("nonce mismatch", func(t *testing.T) {
		a, fp := newSetup(t)
		pre := preAuthState{State: "s", Nonce: "expected-nonce", Path: "/"}
		cookie := sealPreAuthCookie(t, a, pre)
		fp.nextToken = idTokenSpec{
			subject: "user-1", nonce: "wrong-nonce", audience: "test-client",
		}
		req := callbackRequest(t, url.Values{"code": {"c"}, "state": {pre.State}}, cookie)
		w := httptest.NewRecorder()
		a.Callback(w, req)
		assertFailsClosed(t, w)
	})
}
