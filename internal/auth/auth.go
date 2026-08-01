// Package auth drives the OIDC authorization-code flow for a single upstream
// identity provider: a login handler that redirects to the provider, and a
// callback handler that exchanges the code, verifies the ID token, and seals
// a session via the ticket-006 store.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/richpeaua/hlid/internal/config"
	"github.com/richpeaua/hlid/internal/session"
)

// preAuthCookieName is the short-lived cookie binding state+nonce+return path
// across the redirect to the provider and back.
const preAuthCookieName = "hlid_preauth"

// preAuthTTL bounds how long a pre-auth cookie (and thus a login attempt) is
// valid for.
const preAuthTTL = 10 * time.Minute

// defaultReturnPath is used when Login receives no (or an invalid) redirect target.
const defaultReturnPath = "/"

// redirectQueryParam names the query parameter Login reads the post-login
// return path from.
const redirectQueryParam = "redirect_to"

// idClaims holds the well-known ID token claims Callback extracts.
type idClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
}

// preAuthState is the JSON payload sealed (unencrypted, tamper-irrelevant —
// it only ever round-trips through the caller's own browser) into the
// pre-auth cookie.
type preAuthState struct {
	State string `json:"state"`
	Nonce string `json:"nonce"`
	Path  string `json:"path"`
}

// Authenticator drives the OIDC authorization-code flow for one provider.
type Authenticator struct {
	verifier *oidc.IDTokenVerifier
	oauth2   oauth2.Config
	store    *session.Store
	sess     config.Session

	// now is injected for deterministic Expiry in tests; defaults to time.Now.
	now func() time.Time
}

// New builds an Authenticator: OIDC discovery on prov.Issuer, an ID-token verifier bound to
// prov.ClientID, an oauth2.Config (secret read from os.Getenv(prov.ClientSecretEnv)), and the
// session store. ctx is the discovery context.
func New(ctx context.Context, prov config.Provider, sess config.Session, store *session.Store) (*Authenticator, error) {
	secret := os.Getenv(prov.ClientSecretEnv)
	if secret == "" {
		return nil, fmt.Errorf("auth: client secret env %q is empty", prov.ClientSecretEnv)
	}

	provider, err := oidc.NewProvider(ctx, prov.Issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: OIDC discovery on %q: %w", prov.Issuer, err)
	}

	a := &Authenticator{
		store: store,
		sess:  sess,
		now:   time.Now,
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: prov.ClientID})
	a.verifier = verifier

	a.oauth2 = oauth2.Config{
		ClientID:     prov.ClientID,
		ClientSecret: secret,
		RedirectURL:  prov.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       prov.Scopes,
	}

	return a, nil
}

// Login starts the flow: sets a short-lived pre-auth cookie binding random state+nonce, then
// 302-redirects to the provider's authorization endpoint (AuthCodeURL with state + nonce).
func (a *Authenticator) Login(w http.ResponseWriter, r *http.Request) {
	state, err := randomToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	nonce, err := randomToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	path := returnPath(r)

	if err := a.setPreAuthCookie(w, preAuthState{State: state, Nonce: nonce, Path: path}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	authURL := a.oauth2.AuthCodeURL(state, oidc.Nonce(nonce))
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback completes the flow: verifies state against the pre-auth cookie, exchanges the code,
// verifies the ID token + nonce, extracts sub/email, seals a session.Identity (Expiry = now+ttl)
// into the session cookie, clears the pre-auth cookie, and 302-redirects to the original path
// (from the pre-auth cookie, default "/").
func (a *Authenticator) Callback(w http.ResponseWriter, r *http.Request) {
	pre, ok := a.readPreAuthCookie(r)
	if !ok {
		http.Error(w, "missing or invalid pre-auth cookie", http.StatusBadRequest)
		return
	}

	if r.URL.Query().Get("state") != pre.State {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	token, err := a.oauth2.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusUnauthorized)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		http.Error(w, "no id_token in token response", http.StatusUnauthorized)
		return
	}

	idToken, err := a.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, "id token verification failed", http.StatusUnauthorized)
		return
	}

	if idToken.Nonce != pre.Nonce {
		http.Error(w, "nonce mismatch", http.StatusUnauthorized)
		return
	}

	var claims idClaims
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "invalid id token claims", http.StatusUnauthorized)
		return
	}

	identity := session.Identity{
		Subject: claims.Subject,
		Email:   claims.Email,
		Expiry:  a.now().Add(a.sess.TTL),
	}

	sealed, err := a.store.Seal(identity)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     a.sess.CookieName,
		Value:    sealed,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(a.sess.TTL.Seconds()),
	})

	a.clearPreAuthCookie(w)

	http.Redirect(w, r, pre.Path, http.StatusFound)
}

// returnPath extracts the post-login redirect target from the request query,
// falling back to defaultReturnPath. Only a same-origin relative path is
// accepted, to avoid an open redirect.
func returnPath(r *http.Request) string {
	path := r.URL.Query().Get(redirectQueryParam)
	if path == "" || path[0] != '/' || len(path) > 1 && path[1] == '/' {
		return defaultReturnPath
	}
	return path
}

// setPreAuthCookie writes the pre-auth cookie carrying st as a base64url-encoded JSON blob.
func (a *Authenticator) setPreAuthCookie(w http.ResponseWriter, st preAuthState) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("auth: marshal pre-auth state: %w", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     preAuthCookieName,
		Value:    base64.RawURLEncoding.EncodeToString(raw),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(preAuthTTL.Seconds()),
	})
	return nil
}

// readPreAuthCookie decodes the pre-auth cookie from r, if present and well-formed.
func (a *Authenticator) readPreAuthCookie(r *http.Request) (preAuthState, bool) {
	c, err := r.Cookie(preAuthCookieName)
	if err != nil || c.Value == "" {
		return preAuthState{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return preAuthState{}, false
	}
	var st preAuthState
	if err := json.Unmarshal(raw, &st); err != nil {
		return preAuthState{}, false
	}
	if st.State == "" || st.Nonce == "" {
		return preAuthState{}, false
	}
	return st, true
}

// clearPreAuthCookie deletes the pre-auth cookie.
func (a *Authenticator) clearPreAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     preAuthCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// randomToken returns a URL-safe random string suitable for state/nonce use.
func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
