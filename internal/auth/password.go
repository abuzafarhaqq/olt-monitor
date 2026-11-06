// Package auth holds password hashing and session tokens. It knows nothing
// about HTTP or SQL, so both can be tested without either.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// OWASP's argon2id baseline (19 MiB, t=2, p=1). Raising memory is the first
// lever if the login endpoint ever feels too cheap.
const (
	argonMemory  = 19 * 1024
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
	saltLen      = 16
)

var ErrInvalidHash = errors.New("password hash is not a valid argon2id PHC string")

// HashPassword returns a PHC string, which carries its own parameters and salt:
// $argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
//
// Storing the parameters with the hash is what lets the cost be raised later
// without invalidating every existing password.
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", errors.New("password must be at least 8 characters")
	}

	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt: %w", err)
	}

	sum := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

// VerifyPassword compares in constant time. It returns (false, nil) for a
// wrong password and (false, err) only when the stored hash is unusable, so
// callers can tell "wrong" from "this account is broken".
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, hash
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrInvalidHash
	}

	var memory uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}

	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// dummyHash is verified against when a username does not exist, so that a
// failed login takes the same time whether the account is real or not.
var dummyHash string

func init() {
	hash, err := HashPassword("dummy-password-for-timing")
	if err != nil {
		panic(err)
	}
	dummyHash = hash
}

// BurnTime performs the same work as a real verification. Call it when the
// user is unknown; user enumeration by stopwatch is otherwise trivial.
func BurnTime(password string) {
	_, _ = VerifyPassword(password, dummyHash)
}
