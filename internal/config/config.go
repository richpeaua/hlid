// Package config defines Hlid's configuration schema and loads/validates it from YAML.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Route maps a request path prefix to an upstream base URL.
type Route struct {
	Path     string `yaml:"path"`     // e.g. "/app/"; longest-prefix matched by the router
	Upstream string `yaml:"upstream"` // absolute URL, e.g. "http://127.0.0.1:9001"
}

// Config is the whole Hlid configuration (grows in later slices).
type Config struct {
	Listen string  `yaml:"listen"` // listen address, e.g. ":8443"
	Routes []Route `yaml:"routes"`
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

	return nil
}
