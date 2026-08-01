package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/richpeaua/hlid/internal/session"
)

// identityContextKey is the unexported context key type Middleware stores the
// authenticated Identity under.
type identityContextKey struct{}

// exemptPrefixes lists path prefixes Middleware passes through without a session check.
var exemptPrefixes = []string{"/auth/"}

// exemptPaths lists exact paths Middleware passes through without a session check.
var exemptPaths = []string{"/healthz"}

// Middleware gates a handler on a valid, unexpired session cookie. On success it stores the
// Identity in the request context and calls next. On failure it starts login for a browser
// navigation (302 via Login) or returns 401 for a non-navigation request; it never calls next.
// Requests to exempt paths (/healthz, /auth/*) pass through untouched.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isExempt(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		identity, ok := a.identityFromRequest(r)
		if !ok {
			if isNavigation(r) {
				a.Login(w, r)
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), identityContextKey{}, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Handler mounts the auth endpoints: GET /auth/login -> a.Login, GET /auth/callback -> a.Callback.
func (a *Authenticator) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/login", a.Login)
	mux.HandleFunc("GET /auth/callback", a.Callback)
	return mux
}

// IdentityFrom returns the Identity stored by Middleware, if any.
func IdentityFrom(ctx context.Context) (session.Identity, bool) {
	id, ok := ctx.Value(identityContextKey{}).(session.Identity)
	return id, ok
}

// identityFromRequest extracts and validates the session cookie on r: it must be present,
// decrypt via a.store, and be unexpired (Expiry after a.now()).
func (a *Authenticator) identityFromRequest(r *http.Request) (session.Identity, bool) {
	cookie, err := r.Cookie(a.sess.CookieName)
	if err != nil || cookie.Value == "" {
		return session.Identity{}, false
	}

	id, err := a.store.Open(cookie.Value)
	if err != nil {
		return session.Identity{}, false
	}

	if !id.Expiry.After(a.now()) {
		return session.Identity{}, false
	}

	return id, true
}

// isExempt reports whether path bypasses the auth middleware.
func isExempt(path string) bool {
	for _, p := range exemptPaths {
		if path == p {
			return true
		}
	}
	for _, prefix := range exemptPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// isNavigation reports whether r looks like a top-level browser navigation (as opposed to an
// API/XHR/fetch request), based on Sec-Fetch-Mode or the Accept header.
func isNavigation(r *http.Request) bool {
	if mode := r.Header.Get("Sec-Fetch-Mode"); mode != "" {
		return mode == "navigate"
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}
