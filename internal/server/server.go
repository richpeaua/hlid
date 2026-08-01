// Package server assembles Hlid's top-level HTTP server: health endpoint, router, and base
// middleware chain.
package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/richpeaua/hlid/internal/auth"
	"github.com/richpeaua/hlid/internal/config"
	"github.com/richpeaua/hlid/internal/router"
	"github.com/richpeaua/hlid/internal/session"
)

const (
	readHeaderTimeout = 5 * time.Second
	idleTimeout       = 60 * time.Second
)

// New builds the top-level http.Server from cfg; equivalent to NewWithContext with
// context.Background() as the discovery context (see NewWithContext).
func New(cfg *config.Config) (*http.Server, error) {
	return NewWithContext(context.Background(), cfg)
}

// NewWithContext builds the top-level http.Server from cfg: a mux that serves GET /healthz ->
// 200 "ok" and delegates everything else to a router.Router built from cfg.Routes. Applies the
// base middleware chain (request logging placeholder + panic recovery).
//
// When cfg carries auth (both Session and Provider set), it additionally builds the session
// store and Authenticator (ctx bounds OIDC discovery), mounts /auth/ to the Authenticator's
// Handler, and wraps the router with the Authenticator's Middleware so proxied routes require a
// valid session. When neither Session nor Provider is set, the server is assembled without auth
// (Slice-1 shape). Exactly one of Session/Provider set is an error (can't half-configure auth).
//
// Sets Addr from cfg.Listen and sane ReadHeaderTimeout/IdleTimeout.
func NewWithContext(ctx context.Context, cfg *config.Config) (*http.Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("server: invalid config: %w", err)
	}

	if (cfg.Session == nil) != (cfg.Provider == nil) {
		return nil, fmt.Errorf("server: session and provider must both be set or both be unset")
	}

	rt, err := router.New(cfg.Routes)
	if err != nil {
		return nil, fmt.Errorf("server: build router: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	var routed http.Handler = rt

	if cfg.Session != nil && cfg.Provider != nil {
		key, err := session.LoadKey(cfg.Session.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("server: load session key: %w", err)
		}

		store, err := session.NewStore(key)
		if err != nil {
			return nil, fmt.Errorf("server: build session store: %w", err)
		}

		a, err := auth.New(ctx, *cfg.Provider, *cfg.Session, store)
		if err != nil {
			return nil, fmt.Errorf("server: build authenticator: %w", err)
		}

		mux.Handle("/auth/", a.Handler())
		routed = a.Middleware(rt)
	}

	mux.Handle("/", routed)

	handler := chain(mux, logging, recoverPanic)

	return &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}, nil
}

// middleware wraps an http.Handler with additional behavior.
type middleware func(http.Handler) http.Handler

// chain applies middlewares to h in order, so the first middleware listed runs outermost.
func chain(h http.Handler, middlewares ...middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// logging is a request logging placeholder middleware.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// recoverPanic recovers a panic in next, failing closed with a 500 and no stack trace to the
// client.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("recovered panic for %s %s: %v", r.Method, r.URL.Path, rec)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
