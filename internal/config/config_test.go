package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempFile writes contents to a temp file and returns its path.
func writeTempFile(t *testing.T, name, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// TestLoad_ParsesValidYAML covers A1: Load parses a valid YAML file into *Config.
func TestLoad_ParsesValidYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected *Config
	}{
		{
			name: "single route",
			yaml: `
listen: ":8443"
routes:
  - path: /app/
    upstream: http://127.0.0.1:9001
`,
			expected: &Config{
				Listen: ":8443",
				Routes: []Route{
					{Path: "/app/", Upstream: "http://127.0.0.1:9001"},
				},
			},
		},
		{
			name: "multiple routes",
			yaml: `
listen: "0.0.0.0:443"
routes:
  - path: /a/
    upstream: https://127.0.0.1:9001
  - path: /b/
    upstream: https://127.0.0.1:9002
`,
			expected: &Config{
				Listen: "0.0.0.0:443",
				Routes: []Route{
					{Path: "/a/", Upstream: "https://127.0.0.1:9001"},
					{Path: "/b/", Upstream: "https://127.0.0.1:9002"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempFile(t, "config.yaml", tt.yaml)

			got, err := Load(path)
			if err != nil {
				t.Fatalf("Load(%q) returned error: %v", path, err)
			}

			if got.Listen != tt.expected.Listen {
				t.Errorf("Listen = %q, want %q", got.Listen, tt.expected.Listen)
			}
			if len(got.Routes) != len(tt.expected.Routes) {
				t.Fatalf("len(Routes) = %d, want %d", len(got.Routes), len(tt.expected.Routes))
			}
			for i, r := range got.Routes {
				if r != tt.expected.Routes[i] {
					t.Errorf("Routes[%d] = %+v, want %+v", i, r, tt.expected.Routes[i])
				}
			}
		})
	}
}

// TestValidate covers A2: Validate rejects empty listen, empty routes, non-"/" path,
// bad/relative upstream URL, and duplicate paths.
func TestValidate(t *testing.T) {
	validRoute := Route{Path: "/app/", Upstream: "http://127.0.0.1:9001"}

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Listen: ":8443",
				Routes: []Route{validRoute},
			},
			wantErr: false,
		},
		{
			name: "empty listen",
			cfg: Config{
				Listen: "",
				Routes: []Route{validRoute},
			},
			wantErr: true,
		},
		{
			name: "empty routes",
			cfg: Config{
				Listen: ":8443",
				Routes: []Route{},
			},
			wantErr: true,
		},
		{
			name: "non-slash path",
			cfg: Config{
				Listen: ":8443",
				Routes: []Route{{Path: "app/", Upstream: "http://127.0.0.1:9001"}},
			},
			wantErr: true,
		},
		{
			name: "empty path",
			cfg: Config{
				Listen: ":8443",
				Routes: []Route{{Path: "", Upstream: "http://127.0.0.1:9001"}},
			},
			wantErr: true,
		},
		{
			name: "relative upstream URL",
			cfg: Config{
				Listen: ":8443",
				Routes: []Route{{Path: "/app/", Upstream: "/relative/path"}},
			},
			wantErr: true,
		},
		{
			name: "bad upstream URL",
			cfg: Config{
				Listen: ":8443",
				Routes: []Route{{Path: "/app/", Upstream: "not a url \x7f"}},
			},
			wantErr: true,
		},
		{
			name: "unsupported scheme upstream URL",
			cfg: Config{
				Listen: ":8443",
				Routes: []Route{{Path: "/app/", Upstream: "ftp://127.0.0.1:9001"}},
			},
			wantErr: true,
		},
		{
			name: "duplicate paths",
			cfg: Config{
				Listen: ":8443",
				Routes: []Route{
					{Path: "/app/", Upstream: "http://127.0.0.1:9001"},
					{Path: "/app/", Upstream: "http://127.0.0.1:9002"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLoad_Errors covers A3: Load returns a path-wrapped error for a missing file and for
// malformed YAML.
func TestLoad_Errors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "does-not-exist.yaml")

		_, err := Load(path)
		if err == nil {
			t.Fatal("Load() error = nil, want non-nil")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("Load() error = %v, want wrapping os.ErrNotExist", err)
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("Load() error = %q, want it to contain path %q", err.Error(), path)
		}
	})

	t.Run("malformed YAML", func(t *testing.T) {
		path := writeTempFile(t, "bad.yaml", "listen: [this is not valid yaml")

		_, err := Load(path)
		if err == nil {
			t.Fatal("Load() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("Load() error = %q, want it to contain path %q", err.Error(), path)
		}
	})
}
