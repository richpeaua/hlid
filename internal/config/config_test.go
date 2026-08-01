package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// TestLoad_ParsesSessionAndProvider covers A1: Load parses a full config (routes + session +
// provider, TTL like "8h") into *Config, and also parses a valid auth-less config (routes only)
// — both pass Validate.
func TestLoad_ParsesSessionAndProvider(t *testing.T) {
	t.Run("full config with session and provider", func(t *testing.T) {
		yamlContents := `
listen: ":8443"
routes:
  - path: /app/
    upstream: http://127.0.0.1:9001
session:
  cookie_name: hlid_session
  key_file: /etc/hlid/session.key
  ttl: 8h
provider:
  issuer: https://idp.example.com
  client_id: hlid-client
  client_secret_env: HLID_CLIENT_SECRET
  redirect_url: https://hlid.example.com/callback
  scopes: [openid, email]
`
		path := writeTempFile(t, "config.yaml", yamlContents)

		got, err := Load(path)
		if err != nil {
			t.Fatalf("Load(%q) returned error: %v", path, err)
		}

		if got.Session == nil {
			t.Fatal("Session = nil, want non-nil")
		}
		if got.Session.CookieName != "hlid_session" {
			t.Errorf("Session.CookieName = %q, want %q", got.Session.CookieName, "hlid_session")
		}
		if got.Session.KeyFile != "/etc/hlid/session.key" {
			t.Errorf("Session.KeyFile = %q, want %q", got.Session.KeyFile, "/etc/hlid/session.key")
		}
		if got.Session.TTL != 8*time.Hour {
			t.Errorf("Session.TTL = %v, want %v", got.Session.TTL, 8*time.Hour)
		}

		if got.Provider == nil {
			t.Fatal("Provider = nil, want non-nil")
		}
		if got.Provider.Issuer != "https://idp.example.com" {
			t.Errorf("Provider.Issuer = %q, want %q", got.Provider.Issuer, "https://idp.example.com")
		}
		if got.Provider.ClientID != "hlid-client" {
			t.Errorf("Provider.ClientID = %q, want %q", got.Provider.ClientID, "hlid-client")
		}
		if got.Provider.ClientSecretEnv != "HLID_CLIENT_SECRET" {
			t.Errorf("Provider.ClientSecretEnv = %q, want %q", got.Provider.ClientSecretEnv, "HLID_CLIENT_SECRET")
		}
		if got.Provider.RedirectURL != "https://hlid.example.com/callback" {
			t.Errorf("Provider.RedirectURL = %q, want %q", got.Provider.RedirectURL, "https://hlid.example.com/callback")
		}
		if len(got.Provider.Scopes) != 2 || got.Provider.Scopes[0] != "openid" || got.Provider.Scopes[1] != "email" {
			t.Errorf("Provider.Scopes = %v, want %v", got.Provider.Scopes, []string{"openid", "email"})
		}

		if err := got.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("auth-less config with routes only", func(t *testing.T) {
		yamlContents := `
listen: ":8443"
routes:
  - path: /app/
    upstream: http://127.0.0.1:9001
`
		path := writeTempFile(t, "config.yaml", yamlContents)

		got, err := Load(path)
		if err != nil {
			t.Fatalf("Load(%q) returned error: %v", path, err)
		}

		if got.Session != nil {
			t.Errorf("Session = %+v, want nil", got.Session)
		}
		if got.Provider != nil {
			t.Errorf("Provider = %+v, want nil", got.Provider)
		}

		if err := got.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})
}

// TestLoad_ProviderDefaultScopes covers A4: omitted scopes (provider present) defaults to
// [openid, email, profile].
func TestLoad_ProviderDefaultScopes(t *testing.T) {
	yamlContents := `
listen: ":8443"
routes:
  - path: /app/
    upstream: http://127.0.0.1:9001
provider:
  issuer: https://idp.example.com
  client_id: hlid-client
  client_secret_env: HLID_CLIENT_SECRET
  redirect_url: https://hlid.example.com/callback
`
	path := writeTempFile(t, "config.yaml", yamlContents)

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", path, err)
	}

	want := []string{"openid", "email", "profile"}
	if len(got.Provider.Scopes) != len(want) {
		t.Fatalf("Provider.Scopes = %v, want %v", got.Provider.Scopes, want)
	}
	for i, s := range want {
		if got.Provider.Scopes[i] != s {
			t.Errorf("Provider.Scopes[%d] = %q, want %q", i, got.Provider.Scopes[i], s)
		}
	}

	if err := got.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

// TestValidate_Session covers A2: with session present, Validate rejects empty cookie_name,
// empty key_file, non-positive ttl (one subtest each); omitting session entirely is accepted.
func TestValidate_Session(t *testing.T) {
	validRoute := Route{Path: "/app/", Upstream: "http://127.0.0.1:9001"}
	validSession := Session{CookieName: "hlid_session", KeyFile: "/etc/hlid/session.key", TTL: 8 * time.Hour}

	tests := []struct {
		name    string
		session *Session
		wantErr bool
	}{
		{
			name:    "session omitted",
			session: nil,
			wantErr: false,
		},
		{
			name:    "valid session",
			session: &validSession,
			wantErr: false,
		},
		{
			name: "empty cookie_name",
			session: &Session{
				CookieName: "",
				KeyFile:    validSession.KeyFile,
				TTL:        validSession.TTL,
			},
			wantErr: true,
		},
		{
			name: "empty key_file",
			session: &Session{
				CookieName: validSession.CookieName,
				KeyFile:    "",
				TTL:        validSession.TTL,
			},
			wantErr: true,
		},
		{
			name: "non-positive ttl",
			session: &Session{
				CookieName: validSession.CookieName,
				KeyFile:    validSession.KeyFile,
				TTL:        0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Listen:  ":8443",
				Routes:  []Route{validRoute},
				Session: tt.session,
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidate_Provider covers A3: with provider present, Validate rejects non-https/relative
// issuer, empty client_id, empty client_secret_env, bad redirect_url, scopes lacking "openid"
// (one subtest each); omitting provider entirely is accepted.
func TestValidate_Provider(t *testing.T) {
	validRoute := Route{Path: "/app/", Upstream: "http://127.0.0.1:9001"}
	validProvider := Provider{
		Issuer:          "https://idp.example.com",
		ClientID:        "hlid-client",
		ClientSecretEnv: "HLID_CLIENT_SECRET",
		RedirectURL:     "https://hlid.example.com/callback",
		Scopes:          []string{"openid"},
	}

	tests := []struct {
		name     string
		provider *Provider
		wantErr  bool
	}{
		{
			name:     "provider omitted",
			provider: nil,
			wantErr:  false,
		},
		{
			name:     "valid provider",
			provider: &validProvider,
			wantErr:  false,
		},
		{
			name: "relative issuer",
			provider: &Provider{
				Issuer:          "/not/absolute",
				ClientID:        validProvider.ClientID,
				ClientSecretEnv: validProvider.ClientSecretEnv,
				RedirectURL:     validProvider.RedirectURL,
				Scopes:          validProvider.Scopes,
			},
			wantErr: true,
		},
		{
			name: "non-https issuer",
			provider: &Provider{
				Issuer:          "http://idp.example.com",
				ClientID:        validProvider.ClientID,
				ClientSecretEnv: validProvider.ClientSecretEnv,
				RedirectURL:     validProvider.RedirectURL,
				Scopes:          validProvider.Scopes,
			},
			wantErr: true,
		},
		{
			name: "empty client_id",
			provider: &Provider{
				Issuer:          validProvider.Issuer,
				ClientID:        "",
				ClientSecretEnv: validProvider.ClientSecretEnv,
				RedirectURL:     validProvider.RedirectURL,
				Scopes:          validProvider.Scopes,
			},
			wantErr: true,
		},
		{
			name: "empty client_secret_env",
			provider: &Provider{
				Issuer:          validProvider.Issuer,
				ClientID:        validProvider.ClientID,
				ClientSecretEnv: "",
				RedirectURL:     validProvider.RedirectURL,
				Scopes:          validProvider.Scopes,
			},
			wantErr: true,
		},
		{
			name: "bad redirect_url",
			provider: &Provider{
				Issuer:          validProvider.Issuer,
				ClientID:        validProvider.ClientID,
				ClientSecretEnv: validProvider.ClientSecretEnv,
				RedirectURL:     "/relative/callback",
				Scopes:          validProvider.Scopes,
			},
			wantErr: true,
		},
		{
			name: "scopes lacking openid",
			provider: &Provider{
				Issuer:          validProvider.Issuer,
				ClientID:        validProvider.ClientID,
				ClientSecretEnv: validProvider.ClientSecretEnv,
				RedirectURL:     validProvider.RedirectURL,
				Scopes:          []string{"email", "profile"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Listen:   ":8443",
				Routes:   []Route{validRoute},
				Provider: tt.provider,
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
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
