package crypto_test

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/crypto"
)

func newKeyring(t *testing.T) *crypto.Keyring {
	t.Helper()
	key := make([]byte, crypto.KeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("key: %v", err)
	}
	ring, err := crypto.NewKeyring(1, key)
	if err != nil {
		t.Fatalf("keyring: %v", err)
	}
	return ring
}

func TestSealOpenRoundTrip(t *testing.T) {
	ring := newKeyring(t)
	plaintext := []byte("device-password-value")

	sealed, err := ring.Seal(plaintext)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if bytes.Contains(sealed, plaintext) {
		t.Fatal("ciphertext contains the plaintext")
	}

	opened, err := ring.Open(sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !bytes.Equal(opened, plaintext) {
		t.Fatalf("round trip changed the value: %q", opened)
	}
}

func TestSameValueSealsDifferently(t *testing.T) {
	ring := newKeyring(t)
	first, _ := ring.Seal([]byte("same"))
	second, _ := ring.Seal([]byte("same"))
	if bytes.Equal(first, second) {
		t.Fatal("identical ciphertexts: the nonce is not random")
	}
}

func TestTamperedCiphertextRejected(t *testing.T) {
	ring := newKeyring(t)
	sealed, _ := ring.Seal([]byte("value"))

	sealed[len(sealed)-1] ^= 0xff
	if _, err := ring.Open(sealed); err == nil {
		t.Fatal("a tampered ciphertext was accepted: the GCM tag is not checked")
	}
}

func TestWrongKeyFails(t *testing.T) {
	sealed, _ := newKeyring(t).Seal([]byte("value"))
	if _, err := newKeyring(t).Open(sealed); err == nil {
		t.Fatal("ciphertext opened with a different key")
	}
}

func TestKeyLengthEnforced(t *testing.T) {
	if _, err := crypto.NewKeyring(1, make([]byte, 16)); err == nil {
		t.Fatal("a 16-byte key was accepted")
	}
}
