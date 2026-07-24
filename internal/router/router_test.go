package router_test

import (
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/richpeaua/hlid/internal/config"
	"github.com/richpeaua/hlid/internal/router"
)

// newUpstream returns an httptest.Server that writes body and closes it via t.Cleanup.
func newUpstream(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestNew_LongestPrefixDispatchesCorrectUpstream covers A1: with routes "/" and "/app/",
// requests are dispatched to the longest-prefix-matching upstream.
func TestNew_LongestPrefixDispatchesCorrectUpstream(t *testing.T) {
	root := newUpstream(t, "root")
	app := newUpstream(t, "app")

	rt, err := router.New([]config.Route{
		{Path: "/", Upstream: root.URL},
		{Path: "/app/", Upstream: app.URL},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	tests := []struct {
		path string
		want string
	}{
		{path: "/app/x", want: "app"},
		{path: "/other", want: "root"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			rt.ServeHTTP(rec, req)

			if got := rec.Body.String(); got != tt.want {
				t.Errorf("body = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestServeHTTP_NoMatchReturns404 covers A2: no matching route prefix returns 404.
func TestServeHTTP_NoMatchReturns404(t *testing.T) {
	app := newUpstream(t, "app")

	rt, err := router.New([]config.Route{
		{Path: "/app/", Upstream: app.URL},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/other", nil)
	rec := httptest.NewRecorder()

	rt.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestNew_MatchIndependentOfRouteOrder covers A3: shuffling the input route order does not
// change which upstream matches a given request.
func TestNew_MatchIndependentOfRouteOrder(t *testing.T) {
	root := newUpstream(t, "root")
	app := newUpstream(t, "app")
	deep := newUpstream(t, "deep")

	base := []config.Route{
		{Path: "/", Upstream: root.URL},
		{Path: "/app/", Upstream: app.URL},
		{Path: "/app/deep/", Upstream: deep.URL},
	}

	cases := []struct {
		path string
		want string
	}{
		{path: "/app/deep/x", want: "deep"},
		{path: "/app/x", want: "app"},
		{path: "/other", want: "root"},
	}

	rnd := rand.New(rand.NewSource(1))
	for i := 0; i < 5; i++ {
		routes := append([]config.Route(nil), base...)
		rnd.Shuffle(len(routes), func(i, j int) { routes[i], routes[j] = routes[j], routes[i] })

		rt, err := router.New(routes)
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}

		for _, tt := range cases {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			rt.ServeHTTP(rec, req)

			if got := rec.Body.String(); got != tt.want {
				t.Errorf("shuffle %d: path %q body = %q, want %q", i, tt.path, got, tt.want)
			}
		}
	}
}

// TestNew_PropagatesProxyError covers A4: New propagates a proxy.New error for a bad upstream.
func TestNew_PropagatesProxyError(t *testing.T) {
	_, err := router.New([]config.Route{
		{Path: "/app/", Upstream: "not-an-absolute-url"},
	})
	if err == nil {
		t.Fatal("New returned nil error, want non-nil")
	}
}
