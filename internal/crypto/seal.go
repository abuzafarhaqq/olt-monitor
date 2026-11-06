// Package crypto holds the one way this project encrypts secrets at rest.
//
// AES-256-GCM with a random nonce per value. The key never lives in the
// database — it comes from the environment or a systemd credential — because a
// dump plus a key in the same place is a plaintext dump.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
)

// KeySize is fixed by AES-256. Anything else is a configuration error, not a
// weaker cipher.
const KeySize = 32

var ErrInvalidCiphertext = errors.New("ciphertext is malformed or was sealed with another key")

type Keyring struct {
	// Version is stored beside each ciphertext so keys can be rotated without
	// re-encrypting rows in one step.
	Version int
	aead    cipher.AEAD
}

func NewKeyring(version int, key []byte) (*Keyring, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("key must be %d bytes, got %d", KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &Keyring{Version: version, aead: aead}, nil
}

// KeyFromEnv reads a base64-encoded 32-byte key. The variable is read once at
// start-up and never logged.
func KeyFromEnv(name string) ([]byte, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return nil, fmt.Errorf("%s is not set", name)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s is not valid base64: %w", name, err)
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("%s must decode to %d bytes, got %d", name, KeySize, len(key))
	}
	return key, nil
}

// Seal returns nonce||ciphertext. A fresh random nonce per call is what makes
// encrypting the same password twice produce different bytes.
func (k *Keyring) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	return k.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (k *Keyring) Open(sealed []byte) ([]byte, error) {
	nonceSize := k.aead.NonceSize()
	if len(sealed) < nonceSize {
		return nil, ErrInvalidCiphertext
	}
	plaintext, err := k.aead.Open(nil, sealed[:nonceSize], sealed[nonceSize:], nil)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}
	return plaintext, nil
}
