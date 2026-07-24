// Package router dispatches HTTP requests to a proxy handler by longest matching path prefix.
package router

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/richpeaua/hlid/internal/config"
	"github.com/richpeaua/hlid/internal/proxy"
)

// entry pairs a route's path prefix with its constructed proxy handler.
type entry struct {
	path    string
	handler http.Handler
}

// Router dispatches requests to the proxy of the longest-prefix-matching route.
type Router struct {
	entries []entry // sorted longest-prefix-first, then lexicographically
}

// New builds a Router from routes, constructing one proxy handler per route via proxy.New.
// Returns an error if any route's upstream is invalid (propagated from proxy.New).
func New(routes []config.Route) (*Router, error) {
	entries := make([]entry, 0, len(routes))
	for _, rt := range routes {
		handler, err := proxy.New(rt.Upstream)
		if err != nil {
			return nil, fmt.Errorf("router: route %q: %w", rt.Path, err)
		}
		entries = append(entries, entry{path: rt.Path, handler: handler})
	}

	sort.Slice(entries, func(i, j int) bool {
		if len(entries[i].path) != len(entries[j].path) {
			return len(entries[i].path) > len(entries[j].path)
		}
		return entries[i].path < entries[j].path
	})

	return &Router{entries: entries}, nil
}

// ServeHTTP implements http.Handler: longest prefix wins; 404 if no route matches.
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, e := range rt.entries {
		if strings.HasPrefix(r.URL.Path, e.path) {
			e.handler.ServeHTTP(w, r)
			return
		}
	}
	http.NotFound(w, r)
}
