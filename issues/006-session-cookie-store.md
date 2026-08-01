# 006 — session cookie store (AES-256-GCM)

Status: done - https://github.com/richpeaua/hlid/pull/11

Parent: DESIGN.md (components: session; Auth & session decisions) | Slice 2 | Type: AFK

## What to build
An encrypted, authenticated session store: serialize a small identity into an opaque cookie
value and back, using AES-256-GCM. Pure library — no HTTP, no cookies-on-the-wire here (that
wiring is ticket 009).

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

Use only stdlib crypto: `crypto/aes`, `crypto/cipher`, `crypto/rand`, `encoding/base64`,
`encoding/json`, `time`. A fresh random 12-byte GCM nonce per `Seal`, prepended to the
ciphertext; the whole blob is base64url-encoded (no padding) so it is cookie-safe.

## Behavior (no decisions)
- `NewStore` errors unless `len(key) == 32` (AES-256).
- `Seal` produces a different value each call for the same `Identity` (random nonce), and every
  sealed value `Open`s back to the exact `Identity`.
- `Open` returns an error (fail closed) on: a tampered/truncated value, a value sealed under a
  different key, or malformed base64 — never a partially-populated `Identity`.
- `Open` does NOT itself reject an expired `Identity` (callers check `Expiry`); it only decrypts.
- `LoadKey` prefers `HLID_SESSION_KEY` (base64) over `keyFile`; errors if neither yields exactly
  32 bytes after base64-decode. Never logs the key.

## Acceptance
- [ ] `NewStore` rejects a key whose length != 32; accepts a 32-byte key (subtests)
- [ ] Round-trip: `Seal` then `Open` returns the original `Identity` (table-driven over several identities)
- [ ] `Seal` of the same `Identity` twice yields two distinct values that both `Open` correctly
- [ ] `Open` fails closed on a flipped byte, a truncated value, non-base64 input, and a value sealed under a different key (one subtest each)
- [ ] `LoadKey` reads env (base64) with precedence over the file, reads the file otherwise, and errors when the decoded key isn't 32 bytes

## Scope files
- internal/session/session.go
- internal/session/session_test.go

## Blocked by
None — new package, can start immediately.
