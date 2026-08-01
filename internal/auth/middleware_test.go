package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/richpeaua/hlid/internal/config"
	"github.com/richpeaua/hlid/internal/session"
)

// newMiddlewareTestAuthenticator builds an Authenticator wired to a fresh in-memory session
// store and a fixed clock, without any real OIDC discovery (Middleware never drives the OIDC
// flow itself, only Login on failure).
func newMiddlewareTestAuthenticator(t *testing.T, now time.Time) (*Authenticator, *session.Store) {
	t.Helper()

	store, err := session.NewStore(make([]byte, 32))
	if err != nil {
		t.Fatalf("session.NewStore: %v", err)
	}

	a := &Authenticator{
		store: store,
		sess:  testSession(),
		now:   func() time.Time { return now },
	}
	return a, store
}

func sealSessionCookie(t *testing.T, store *session.Store, sess config.Session, id session.Identity) *http.Cookie {
	t.Helper()
	sealed, err := store.Seal(id)
	if err != nil {
		t.Fatalf("store.Seal: %v", err)
	}
	return &http.Cookie{Name: sess.CookieName, Value: sealed}
}

// A1: a valid unexpired session cookie passes through to the wrapped handler and IdentityFrom
// returns the Identity.
func TestMiddlewareValidSessionPassesThrough(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	a, store := newMiddlewareTestAuthenticator(t, now)

	wantID := session.Identity{Subject: "user-1", Email: "user-1@example.com", Expiry: now.Add(time.Hour)}
	cookie := sealSessionCookie(t, store, a.sess, wantID)

	var calledNext bool
	var gotID session.Identity
	var gotOK bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		gotID, gotOK = IdentityFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/app/dashboard", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	a.Middleware(next).ServeHTTP(rec, req)

	if !calledNext {
		t.Fatal("expected next to be called for a valid session")
	}
	if !gotOK {
		t.Fatal("IdentityFrom returned ok=false inside next")
	}
	if gotID != wantID {
		t.Errorf("IdentityFrom = %+v, want %+v", gotID, wantID)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// A2: missing/invalid/expired cookie yields 302 /auth/login for a navigation request and 401
// for a non-navigation request, and never calls next.
func TestMiddlewareDeniesUnauthenticated(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	newNeverCalledNext := func(t *testing.T) http.Handler {
		t.Helper()
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next must not be called on a denied request")
		})
	}

	cases := []struct {
		name    string
		cookie  func(a *Authenticator, store *session.Store) *http.Cookie
		headers func(r *http.Request)
		wantNav int // status for a navigation request
	}{
		{
			name:   "missing cookie",
			cookie: func(a *Authenticator, store *session.Store) *http.Cookie { return nil },
		},
		{
			name: "invalid cookie",
			cookie: func(a *Authenticator, store *session.Store) *http.Cookie {
				return &http.Cookie{Name: a.sess.CookieName, Value: "not-a-valid-sealed-value"}
			},
		},
		{
			name: "expired cookie",
			cookie: func(a *Authenticator, store *session.Store) *http.Cookie {
				return sealSessionCookie(t, store, a.sess, session.Identity{
					Subject: "user-1",
					Expiry:  now.Add(-time.Minute), // already expired
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/navigation", func(t *testing.T) {
			a, store := newMiddlewareTestAuthenticator(t, now)
			req := httptest.NewRequest(http.MethodGet, "/app/dashboard", nil)
			req.Header.Set("Sec-Fetch-Mode", "navigate")
			if c := tc.cookie(a, store); c != nil {
				req.AddCookie(c)
			}
			rec := httptest.NewRecorder()

			a.Middleware(newNeverCalledNext(t)).ServeHTTP(rec, req)

			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
			}
			loc := rec.Header().Get("Location")
			if loc == "" {
				t.Fatal("expected a Location header pointing at the login flow")
			}
		})

		t.Run(tc.name+"/non-navigation", func(t *testing.T) {
			a, store := newMiddlewareTestAuthenticator(t, now)
			req := httptest.NewRequest(http.MethodGet, "/app/dashboard", nil)
			req.Header.Set("Accept", "application/json")
			if c := tc.cookie(a, store); c != nil {
				req.AddCookie(c)
			}
			rec := httptest.NewRecorder()

			a.Middleware(newNeverCalledNext(t)).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

// Exempt paths (/healthz, /auth/*) pass through untouched, with no session cookie required.
func TestMiddlewareExemptPathsPassThrough(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	a, _ := newMiddlewareTestAuthenticator(t, now)

	for _, path := range []string{"/healthz", "/auth/login", "/auth/callback"} {
		t.Run(path, func(t *testing.T) {
			var calledNext bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calledNext = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			a.Middleware(next).ServeHTTP(rec, req)

			if !calledNext {
				t.Fatalf("expected exempt path %q to reach next", path)
			}
		})
	}
}
