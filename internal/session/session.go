// Package session implements an encrypted, authenticated session store: it
// serializes a small identity into an opaque cookie value and back, using
// AES-256-GCM. It is a pure library — no HTTP, no cookies-on-the-wire here.
package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// keySize is the required AES-256 key length in bytes.
const keySize = 32

// envSessionKey is the environment variable LoadKey prefers, base64-encoded.
const envSessionKey = "HLID_SESSION_KEY"

// Identity is the authenticated subject persisted in the session.
type Identity struct {
	Subject string    `json:"sub"`             // OIDC subject (stable user id)
	Email   string    `json:"email,omitempty"` // best-effort display email
	Expiry  time.Time `json:"exp"`             // absolute session expiry
}

// Store seals and opens session values with a fixed AES-256 key.
type Store struct {
	aead cipher.AEAD
}

// NewStore builds a Store from a 32-byte AES-256 key.
func NewStore(key []byte) (*Store, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("session: key must be %d bytes, got %d", keySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("session: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("session: new GCM: %w", err)
	}
	return &Store{aead: aead}, nil
}

// Seal encrypts+authenticates id into an opaque, URL-safe cookie value.
func (s *Store) Seal(id Identity) (string, error) {
	plaintext, err := json.Marshal(id)
	if err != nil {
		return "", fmt.Errorf("session: marshal identity: %w", err)
	}

	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("session: generate nonce: %w", err)
	}

	sealed := s.aead.Seal(nil, nonce, plaintext, nil)
	blob := append(nonce, sealed...)

	return base64.RawURLEncoding.EncodeToString(blob), nil
}

// Open authenticates+decrypts a cookie value back into an Identity.
//
// Open fails closed: on a tampered/truncated value, a value sealed under a
// different key, or malformed base64, it returns an error and a zero-value
// Identity — never a partially-populated one. It does not itself reject an
// expired Identity; callers must check Expiry.
func (s *Store) Open(value string) (Identity, error) {
	blob, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Identity{}, fmt.Errorf("session: decode value: %w", err)
	}

	nonceSize := s.aead.NonceSize()
	if len(blob) < nonceSize {
		return Identity{}, fmt.Errorf("session: value too short")
	}
	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]

	plaintext, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return Identity{}, fmt.Errorf("session: open: %w", err)
	}

	var id Identity
	if err := json.Unmarshal(plaintext, &id); err != nil {
		return Identity{}, fmt.Errorf("session: unmarshal identity: %w", err)
	}
	return id, nil
}

// LoadKey reads the 32-byte key: env HLID_SESSION_KEY (base64) if set, else
// base64 in keyFile. It errors if neither yields exactly 32 bytes after
// base64-decode. It never logs the key.
func LoadKey(keyFile string) ([]byte, error) {
	if encoded := os.Getenv(envSessionKey); encoded != "" {
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("session: decode %s: %w", envSessionKey, err)
		}
		if len(key) != keySize {
			return nil, fmt.Errorf("session: %s must decode to %d bytes, got %d", envSessionKey, keySize, len(key))
		}
		return key, nil
	}

	raw, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("session: read key file %q: %w", keyFile, err)
	}
	key, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		return nil, fmt.Errorf("session: decode key file %q: %w", keyFile, err)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("session: key file %q must decode to %d bytes, got %d", keyFile, keySize, len(key))
	}
	return key, nil
}
