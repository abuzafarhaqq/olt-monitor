package auth_test

import (
	"strings"
	"testing"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/auth"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash is not an argon2id PHC string: %s", hash)
	}
	if strings.Contains(hash, "correct horse") {
		t.Fatal("hash contains the password")
	}

	ok, err := auth.VerifyPassword("correct horse battery staple", hash)
	if err != nil || !ok {
		t.Fatalf("verify correct password: ok=%v err=%v", ok, err)
	}

	ok, err = auth.VerifyPassword("correct horse battery stapl", hash)
	if err != nil {
		t.Fatalf("verify wrong password returned error: %v", err)
	}
	if ok {
		t.Fatal("wrong password verified")
	}
}

func TestSaltChangesEveryHash(t *testing.T) {
	first, err := auth.HashPassword("same-password-twice")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	second, err := auth.HashPassword("same-password-twice")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if first == second {
		t.Fatal("two hashes of the same password are identical: salt is not random")
	}
}

func TestShortPasswordRejected(t *testing.T) {
	if _, err := auth.HashPassword("short"); err == nil {
		t.Fatal("a 5-character password was accepted")
	}
}

func TestMalformedHashIsAnError(t *testing.T) {
	cases := map[string]string{
		"empty":        "",
		"not phc":      "plaintext",
		"wrong algo":   "$bcrypt$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA",
		"missing part": "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA",
	}
	for name, encoded := range cases {
		ok, err := auth.VerifyPassword("whatever", encoded)
		if ok || err == nil {
			t.Errorf("%s: expected an error, got ok=%v err=%v", name, ok, err)
		}
	}
}

func TestSessionTokensAreUniqueAndHashed(t *testing.T) {
	first, hashA, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	second, hashB, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	if first == second {
		t.Fatal("two session tokens are identical")
	}
	if string(hashA) == first {
		t.Fatal("the stored value is the token itself")
	}
	if len(hashA) != 32 {
		t.Fatalf("hash length = %d, want sha-256", len(hashA))
	}
	if string(hashA) == string(hashB) {
		t.Fatal("different tokens produced the same hash")
	}
	if string(auth.HashToken(first)) != string(hashA) {
		t.Fatal("HashToken is not stable")
	}
}
