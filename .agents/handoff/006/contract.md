# Contract — 006 — session cookie store (AES-256-GCM)

ticket: issues/006-session-cookie-store.md
design_refs: DESIGN.md (components: session; Auth & session decisions)
scope_files:
  - internal/session/session.go
  - internal/session/session_test.go

## Interface (verbatim from ticket)
```
package session
// Identity is the authenticated subject persisted in the session.
type Identity struct {
    Subject string    `json:"sub"`             // OIDC subject (stable user id)
    Email   string    `json:"email,omitempty"`
    Expiry  time.Time `json:"exp"`             // absolute session expiry
}
// Store seals and opens session values with a fixed AES-256 key.
type Store struct { /* unexported: aead cipher.AEAD */ }
// NewStore builds a Store from a 32-byte AES-256 key.
func NewStore(key []byte) (*Store, error)
// Seal encrypts+authenticates id into an opaque, URL-safe cookie value.
func (s *Store) Seal(id Identity) (string, error)
// Open authenticates+decrypts a cookie value back into an Identity.
func (s *Store) Open(value string) (Identity, error)
// LoadKey reads the 32-byte key: env HLID_SESSION_KEY (base64) if set, else base64 in keyFile.
func LoadKey(keyFile string) ([]byte, error)
```

## Acceptance
- [ ] A1: `NewStore` rejects a key whose length != 32; accepts a 32-byte key (subtests)
- [ ] A2: Round-trip: `Seal` then `Open` returns the original `Identity` (table-driven over several identities)
- [ ] A3: `Seal` of the same `Identity` twice yields two distinct values that both `Open` correctly
- [ ] A4: `Open` fails closed on a flipped byte, a truncated value, non-base64 input, and a value sealed under a different key (one subtest each)
- [ ] A5: `LoadKey` reads env (base64) with precedence over the file, reads the file otherwise, and errors when the decoded key isn't 32 bytes

## Constraints
See .agents/STANDARDS.md for the binding coding/determinism/purity rules.

## Handoff log
Round records accumulate beside this file: implement-rN.md (implementer record) and review-rN.md (reviewer findings, ending in `VERDICT: APPROVE|REQUEST-CHANGES`).
