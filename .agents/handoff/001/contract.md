# Contract — 001 — config schema + loader

ticket: issues/001-config-schema-loader.md
design_refs: DESIGN.md (Config schema, components)
scope_files:
  - internal/config/config.go
  - internal/config/config_test.go

## Interface (verbatim from ticket)
```
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
```

## Acceptance
- [ ] A1: `Load` parses a valid YAML file into `*Config` (table-driven test with a temp file)
- [ ] A2: `Validate` rejects empty listen, empty routes, non-`/` path, bad/relative upstream URL, and duplicate paths (one subtest each)
- [ ] A3: `Load` returns a path-wrapped error for a missing file and for malformed YAML

## Constraints
See .agents/STANDARDS.md for the binding coding/determinism/purity rules.

## Handoff log
Round records accumulate beside this file: implement-rN.md (implementer record) and review-rN.md (reviewer findings, ending in `VERDICT: APPROVE|REQUEST-CHANGES`).
