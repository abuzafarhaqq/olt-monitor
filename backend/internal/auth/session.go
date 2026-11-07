package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

// SessionTTL is how long a login lasts. Short enough that a stolen cookie
// expires, long enough for a NOC shift.
const SessionTTL = 12 * time.Hour

const tokenBytes = 32

// NewSessionToken returns the value that goes into the cookie and the hash that
// goes into the database. The raw token is never stored: a database dump yields
// hashes, and a hash cannot be replayed as a cookie.
func NewSessionToken() (token string, hash []byte, err error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
