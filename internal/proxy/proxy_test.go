package proxy_test

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/richpeaua/hlid/internal/proxy"
)

// TestNew_ForwardsRequest is the named test for A1: New forwards method+path+query to an
// httptest.Server upstream and returns its response.
func TestNew_ForwardsRequest(t *testing.T) {
	const wantBody = "upstream response body"

	var gotMethod, gotPath, gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(wantBody))
	}))
	defer upstream.Close()

	handler, err := proxy.New(upstream.URL)
	if err != nil {
		t.Fatalf("New(%q) returned error: %v", upstream.URL, err)
	}

	frontend := httptest.NewServer(handler)
	defer frontend.Close()

	resp, err := http.Post(frontend.URL+"/some/path?foo=bar", "text/plain", nil)
	if err != nil {
		t.Fatalf("request to proxy failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if string(body) != wantBody {
		t.Errorf("body = %q, want %q", string(body), wantBody)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("upstream method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotPath != "/some/path" {
		t.Errorf("upstream path = %q, want %q", gotPath, "/some/path")
	}
	if gotQuery != "foo=bar" {
		t.Errorf("upstream query = %q, want %q", gotQuery, "foo=bar")
	}
}

// TestNew_UnreachableUpstreamReturns502 is the named test for A2: an unreachable upstream (a
// closed port) returns 502 via the proxy's ErrorHandler, not a panic or 500.
func TestNew_UnreachableUpstreamReturns502(t *testing.T) {
	// Bind a listener then close it immediately to obtain a port nothing is listening on.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate a port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("failed to close listener: %v", err)
	}

	handler, err := proxy.New("http://" + addr)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	frontend := httptest.NewServer(handler)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL + "/anything")
	if err != nil {
		t.Fatalf("request to proxy failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadGateway)
	}
}

// TestNew_InvalidUpstream is the named test for A3: New errors on empty and relative upstream
// URLs.
func TestNew_InvalidUpstream(t *testing.T) {
	tests := []struct {
		name     string
		upstream string
	}{
		{name: "empty", upstream: ""},
		{name: "relative", upstream: "/just/a/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := proxy.New(tt.upstream)
			if err == nil {
				t.Fatalf("New(%q) returned nil error, want non-nil", tt.upstream)
			}
			if handler != nil {
				t.Errorf("New(%q) returned non-nil handler alongside error", tt.upstream)
			}
		})
	}
}
