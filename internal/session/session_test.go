package session

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A1: NewStore rejects a key whose length != 32; accepts a 32-byte key.
func TestNewStore(t *testing.T) {
	cases := []struct {
		name    string
		keyLen  int
		wantErr bool
	}{
		{"too short", 16, true},
		{"too long", 33, true},
		{"empty", 0, true},
		{"exact 32", 32, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			key := make([]byte, c.keyLen)
			store, err := NewStore(key)
			if c.wantErr {
				if err == nil {
					t.Fatalf("NewStore(len=%d) = nil error, want error", c.keyLen)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewStore(len=%d) unexpected error: %v", c.keyLen, err)
			}
			if store == nil {
				t.Fatalf("NewStore(len=%d) returned nil store", c.keyLen)
			}
		})
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	store, err := NewStore(key)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

// A2: Round-trip: Seal then Open returns the original Identity.
func TestSealOpenRoundTrip(t *testing.T) {
	store := newTestStore(t)
	exp := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	cases := []struct {
		name string
		id   Identity
	}{
		{"basic", Identity{Subject: "user-1", Email: "a@example.com", Expiry: exp}},
		{"no email", Identity{Subject: "user-2", Expiry: exp}},
		{"empty subject", Identity{Subject: "", Email: "b@example.com", Expiry: exp}},
		{"zero expiry", Identity{Subject: "user-3", Email: "c@example.com"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sealed, err := store.Seal(c.id)
			if err != nil {
				t.Fatalf("Seal: %v", err)
			}
			got, err := store.Open(sealed)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if got.Subject != c.id.Subject || got.Email != c.id.Email || !got.Expiry.Equal(c.id.Expiry) {
				t.Fatalf("Open() = %+v, want %+v", got, c.id)
			}
		})
	}
}

// A3: Seal of the same Identity twice yields two distinct values that both Open correctly.
func TestSealIsRandomized(t *testing.T) {
	store := newTestStore(t)
	id := Identity{Subject: "user-1", Email: "a@example.com", Expiry: time.Now().UTC()}

	v1, err := store.Seal(id)
	if err != nil {
		t.Fatalf("Seal (1): %v", err)
	}
	v2, err := store.Seal(id)
	if err != nil {
		t.Fatalf("Seal (2): %v", err)
	}
	if v1 == v2 {
		t.Fatalf("Seal produced identical values for two calls: %q", v1)
	}

	for _, v := range []string{v1, v2} {
		got, err := store.Open(v)
		if err != nil {
			t.Fatalf("Open(%q): %v", v, err)
		}
		if got.Subject != id.Subject || got.Email != id.Email || !got.Expiry.Equal(id.Expiry) {
			t.Fatalf("Open(%q) = %+v, want %+v", v, got, id)
		}
	}
}

// A4: Open fails closed on a flipped byte, a truncated value, non-base64 input,
// and a value sealed under a different key.
func TestOpenFailsClosed(t *testing.T) {
	store := newTestStore(t)
	id := Identity{Subject: "user-1", Email: "a@example.com", Expiry: time.Now().UTC()}

	sealed, err := store.Seal(id)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	t.Run("flipped byte", func(t *testing.T) {
		raw, err := base64.RawURLEncoding.DecodeString(sealed)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(raw) == 0 {
			t.Fatal("sealed value decoded to empty")
		}
		raw[len(raw)-1] ^= 0xFF
		tampered := base64.RawURLEncoding.EncodeToString(raw)

		if _, err := store.Open(tampered); err == nil {
			t.Fatal("Open(tampered) = nil error, want error")
		}
	})

	t.Run("truncated", func(t *testing.T) {
		truncated := sealed[:len(sealed)/2]
		if _, err := store.Open(truncated); err == nil {
			t.Fatal("Open(truncated) = nil error, want error")
		}
	})

	t.Run("non-base64", func(t *testing.T) {
		if _, err := store.Open("not!!valid$$base64%%"); err == nil {
			t.Fatal("Open(non-base64) = nil error, want error")
		}
	})

	t.Run("different key", func(t *testing.T) {
		otherKey := make([]byte, 32)
		for i := range otherKey {
			otherKey[i] = byte(255 - i)
		}
		other, err := NewStore(otherKey)
		if err != nil {
			t.Fatalf("NewStore(other): %v", err)
		}
		if _, err := other.Open(sealed); err == nil {
			t.Fatal("Open(sealed under different key) = nil error, want error")
		}
	})

	t.Run("empty result never populated on error", func(t *testing.T) {
		got, err := store.Open("not!!valid$$base64%%")
		if err == nil {
			t.Fatal("expected error")
		}
		zero := Identity{}
		if got != zero {
			t.Fatalf("Open() on error = %+v, want zero value %+v", got, zero)
		}
	})
}

// A5: LoadKey reads env (base64) with precedence over the file, reads the file
// otherwise, and errors when the decoded key isn't 32 bytes.
func TestLoadKey(t *testing.T) {
	const envVar = "HLID_SESSION_KEY"

	mkKey := func(b byte) []byte {
		key := make([]byte, 32)
		for i := range key {
			key[i] = b
		}
		return key
	}

	writeKeyFile := func(t *testing.T, dir string, key []byte) string {
		t.Helper()
		path := filepath.Join(dir, "session.key")
		if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		return path
	}

	t.Run("env takes precedence over file", func(t *testing.T) {
		dir := t.TempDir()
		envKey := mkKey(1)
		fileKey := mkKey(2)
		keyFile := writeKeyFile(t, dir, fileKey)

		t.Setenv(envVar, base64.StdEncoding.EncodeToString(envKey))

		got, err := LoadKey(keyFile)
		if err != nil {
			t.Fatalf("LoadKey: %v", err)
		}
		if string(got) != string(envKey) {
			t.Fatalf("LoadKey() = %x, want env key %x", got, envKey)
		}
	})

	t.Run("falls back to file when env unset", func(t *testing.T) {
		dir := t.TempDir()
		fileKey := mkKey(3)
		keyFile := writeKeyFile(t, dir, fileKey)

		t.Setenv(envVar, "")
		os.Unsetenv(envVar)

		got, err := LoadKey(keyFile)
		if err != nil {
			t.Fatalf("LoadKey: %v", err)
		}
		if string(got) != string(fileKey) {
			t.Fatalf("LoadKey() = %x, want file key %x", got, fileKey)
		}
	})

	t.Run("errors when env key is not 32 bytes", func(t *testing.T) {
		dir := t.TempDir()
		keyFile := writeKeyFile(t, dir, mkKey(4))
		t.Setenv(envVar, base64.StdEncoding.EncodeToString([]byte("short")))

		if _, err := LoadKey(keyFile); err == nil {
			t.Fatal("LoadKey() = nil error, want error for short env key")
		}
	})

	t.Run("errors when file key is not 32 bytes", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "session.key")
		if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString([]byte("short"))), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		os.Unsetenv(envVar)

		if _, err := LoadKey(path); err == nil {
			t.Fatal("LoadKey() = nil error, want error for short file key")
		}
	})

	t.Run("errors when neither env nor file is available", func(t *testing.T) {
		os.Unsetenv(envVar)
		if _, err := LoadKey(filepath.Join(t.TempDir(), "missing.key")); err == nil {
			t.Fatal("LoadKey() = nil error, want error when neither source available")
		}
	})

	t.Run("errors on malformed base64 in env", func(t *testing.T) {
		dir := t.TempDir()
		keyFile := writeKeyFile(t, dir, mkKey(5))
		t.Setenv(envVar, "not-valid-base64!!")

		if _, err := LoadKey(keyFile); err == nil {
			t.Fatal("LoadKey() = nil error, want error for malformed base64 env value")
		}
	})
}
