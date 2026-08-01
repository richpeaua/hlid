// Package config defines Hlid's configuration schema and loads/validates it from YAML.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// defaultScopes is applied to a Provider when Scopes is omitted from config.
var defaultScopes = []string{"openid", "email", "profile"}

// Route maps a request path prefix to an upstream base URL.
type Route struct {
	Path     string `yaml:"path"`     // e.g. "/app/"; longest-prefix matched by the router
	Upstream string `yaml:"upstream"` // absolute URL, e.g. "http://127.0.0.1:9001"
}

// Session configures the encrypted session cookie.
type Session struct {
	CookieName string        `yaml:"cookie_name"` // e.g. "hlid_session"
	KeyFile    string        `yaml:"key_file"`    // path to base64 32-byte key (env HLID_SESSION_KEY overrides)
	TTL        time.Duration `yaml:"ttl"`         // session lifetime, e.g. 8h
}

// Provider configures the single upstream OIDC identity provider.
type Provider struct {
	Issuer          string   `yaml:"issuer"` // OIDC discovery base URL (https)
	ClientID        string   `yaml:"client_id"`
	ClientSecretEnv string   `yaml:"client_secret_env"` // env var NAME holding the secret
	RedirectURL     string   `yaml:"redirect_url"`      // absolute https callback URL
	Scopes          []string `yaml:"scopes"`            // must include "openid"
}

// Config is the whole Hlid configuration (grows in later slices).
type Config struct {
	Listen   string    `yaml:"listen"` // listen address, e.g. ":8443"
	Routes   []Route   `yaml:"routes"`
	Session  *Session  `yaml:"session"`  // optional; auth is not mandatory at the config layer
	Provider *Provider `yaml:"provider"` // optional; auth is not mandatory at the config layer
}

// Load reads, parses, and validates the YAML config at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load config %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("load config %q: %w", path, err)
	}

	if cfg.Provider != nil && len(cfg.Provider.Scopes) == 0 {
		cfg.Provider.Scopes = append([]string(nil), defaultScopes...)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("load config %q: %w", path, err)
	}

	return &cfg, nil
}

// Validate checks the config is internally consistent; returns a descriptive error otherwise.
func (c *Config) Validate() error {
	if c.Listen == "" {
		return fmt.Errorf("listen: must not be empty")
	}

	if len(c.Routes) == 0 {
		return fmt.Errorf("routes: must not be empty")
	}

	seen := make(map[string]bool, len(c.Routes))
	for i, r := range c.Routes {
		if r.Path == "" || !strings.HasPrefix(r.Path, "/") {
			return fmt.Errorf("routes[%d].path %q: must start with %q", i, r.Path, "/")
		}

		if seen[r.Path] {
			return fmt.Errorf("routes[%d].path %q: duplicate route path", i, r.Path)
		}
		seen[r.Path] = true

		u, err := url.Parse(r.Upstream)
		if err != nil {
			return fmt.Errorf("routes[%d].upstream %q: %w", i, r.Upstream, err)
		}
		if !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("routes[%d].upstream %q: must be an absolute http(s) URL", i, r.Upstream)
		}
	}

	if c.Session != nil {
		if err := c.Session.validate(); err != nil {
			return err
		}
	}

	if c.Provider != nil {
		if err := c.Provider.validate(); err != nil {
			return err
		}
	}

	return nil
}

// validate checks the session config is internally consistent.
func (s *Session) validate() error {
	if s.CookieName == "" {
		return fmt.Errorf("session.cookie_name: must not be empty")
	}
	if s.KeyFile == "" {
		return fmt.Errorf("session.key_file: must not be empty")
	}
	if s.TTL <= 0 {
		return fmt.Errorf("session.ttl: must be positive, got %s", s.TTL)
	}
	return nil
}

// validate checks the provider config is internally consistent.
func (p *Provider) validate() error {
	issuerURL, err := url.Parse(p.Issuer)
	if err != nil || !issuerURL.IsAbs() || issuerURL.Scheme != "https" || issuerURL.Host == "" {
		return fmt.Errorf("provider.issuer %q: must be an absolute https URL", p.Issuer)
	}

	if p.ClientID == "" {
		return fmt.Errorf("provider.client_id: must not be empty")
	}

	if p.ClientSecretEnv == "" {
		return fmt.Errorf("provider.client_secret_env: must not be empty")
	}

	redirectURL, err := url.Parse(p.RedirectURL)
	if err != nil || !redirectURL.IsAbs() || (redirectURL.Scheme != "http" && redirectURL.Scheme != "https") || redirectURL.Host == "" {
		return fmt.Errorf("provider.redirect_url %q: must be an absolute http(s) URL", p.RedirectURL)
	}

	hasOpenID := false
	for _, scope := range p.Scopes {
		if scope == "openid" {
			hasOpenID = true
			break
		}
	}
	if !hasOpenID {
		return fmt.Errorf("provider.scopes: must contain %q", "openid")
	}

	return nil
}
