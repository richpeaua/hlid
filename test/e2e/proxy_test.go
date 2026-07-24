// Package e2e contains black-box integration tests that exercise the assembled Hlid server
// end-to-end: config -> server -> router -> proxy -> upstream. No internals are mocked.
package e2e

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richpeaua/hlid/internal/config"
	"github.com/richpeaua/hlid/internal/server"
)

// newUpstream returns an httptest.Server that echoes name in its response body, so tests can
// assert which upstream actually handled a request.
func newUpstream(t *testing.T, name string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s:%s", name, r.URL.Path)
	}))
}

// writeConfig writes a Hlid YAML config routing "/app/" to appUpstream and "/" to rootUpstream,
// returning the config file's path.
func writeConfig(t *testing.T, appUpstream, rootUpstream string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hlid.yaml")
	body := fmt.Sprintf(
		"listen: \":0\"\nroutes:\n  - path: \"/app/\"\n    upstream: %q\n  - path: \"/\"\n    upstream: %q\n",
		appUpstream, rootUpstream,
	)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// buildHandler loads cfgPath and assembles the Hlid handler via server.New, the same assembly
// path main() uses.
func buildHandler(t *testing.T, cfgPath string) http.Handler {
	t.Helper()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	return srv.Handler
}

// TestE2E_RoutesToUpstream proves A1: longest-prefix routing holds end-to-end and each
// upstream's response body reaches the client verbatim.
func TestE2E_RoutesToUpstream(t *testing.T) {
	appUpstream := newUpstream(t, "app")
	defer appUpstream.Close()
	rootUpstream := newUpstream(t, "root")
	defer rootUpstream.Close()

	cfgPath := writeConfig(t, appUpstream.URL, rootUpstream.URL)
	handler := buildHandler(t, cfgPath)

	hlid := httptest.NewServer(handler)
	defer hlid.Close()

	cases := []struct {
		path string
		want string
	}{
		{"/app/x", "app:/app/x"},
		{"/y", "root:/y"},
	}
	for _, tc := range cases {
		resp, err := http.Get(hlid.URL + tc.path)
		if err != nil {
			t.Fatalf("GET %s: %v", tc.path, err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("read body for %s: %v", tc.path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", tc.path, resp.StatusCode)
		}
		if string(body) != tc.want {
			t.Errorf("%s: body = %q, want %q", tc.path, body, tc.want)
		}
	}
}

// TestE2E_Healthz proves A2: GET /healthz returns 200 "ok" end-to-end.
func TestE2E_Healthz(t *testing.T) {
	appUpstream := newUpstream(t, "app")
	defer appUpstream.Close()
	rootUpstream := newUpstream(t, "root")
	defer rootUpstream.Close()

	cfgPath := writeConfig(t, appUpstream.URL, rootUpstream.URL)
	handler := buildHandler(t, cfgPath)

	hlid := httptest.NewServer(handler)
	defer hlid.Close()

	resp, err := http.Get(hlid.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

// TestE2E_UnroutedPath proves A3: an unrouted path returns 404 end-to-end. Both configured
// routes ("/app/" and "/") are prefixes of "/", so this uses a config with no root catch-all to
// exercise the miss path.
func TestE2E_UnroutedPath(t *testing.T) {
	appUpstream := newUpstream(t, "app")
	defer appUpstream.Close()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "hlid.yaml")
	body := fmt.Sprintf("listen: \":0\"\nroutes:\n  - path: \"/app/\"\n    upstream: %q\n", appUpstream.URL)
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	handler := buildHandler(t, cfgPath)

	hlid := httptest.NewServer(handler)
	defer hlid.Close()

	resp, err := http.Get(hlid.URL + "/nope")
	if err != nil {
		t.Fatalf("GET /nope: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// TestE2E_MainBuildsAndFailsFastOnInvalidConfig proves A4: `go build ./cmd/hlid` succeeds and
// the resulting binary exits non-zero when pointed at a missing/invalid config.
func TestE2E_MainBuildsAndFailsFastOnInvalidConfig(t *testing.T) {
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

	run := exec.Command(binPath, "-config", filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	out, err := run.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for invalid config, got success; output:\n%s", out)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() == 0 {
		t.Fatalf("exit code = 0, want non-zero")
	}
	if !strings.Contains(string(out), "config") {
		t.Errorf("output = %q, want it to mention the config failure", out)
	}
}
