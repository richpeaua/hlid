# 001 — config schema + loader

Status: todo
Parent: DESIGN.md (Config schema, components) | Slice 1 | Type: AFK

## What to build
The typed config and its loader/validator. YAML on disk -> validated `*Config`.

    package config

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
    func Load(path string) (*Config, error)

    // Validate checks the config is internally consistent; returns a descriptive error otherwise.
    func (c *Config) Validate() error

Use `gopkg.in/yaml.v3` (run `go get gopkg.in/yaml.v3`; commit go.mod + go.sum).

## Behavior (no decisions)
- `Load` wraps parse/read errors with the path (`%w`).
- `Validate` fails when: `Listen` is empty; `Routes` is empty; any `Route.Path` is empty or does
  not start with `/`; any `Route.Upstream` is not a parseable absolute `http`/`https` URL.
- `Validate` fails on duplicate `Route.Path` values.
- A valid config round-trips: `Load` of a well-formed file returns the expected struct.

## Acceptance
- [ ] `Load` parses a valid YAML file into `*Config` (table-driven test with a temp file)
- [ ] `Validate` rejects empty listen, empty routes, non-`/` path, bad/relative upstream URL, and duplicate paths (one subtest each)
- [ ] `Load` returns a path-wrapped error for a missing file and for malformed YAML

## Scope files
- internal/config/config.go
- internal/config/config_test.go

## Blocked by
None — can start immediately.
