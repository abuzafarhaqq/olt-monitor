package api_test

import (
	"context"
	"net/http"
	"testing"
)

// The endpoint must answer the same way for an address that exists and one
// that does not: identical status, identical empty body.
func TestPasswordResetIsNotAnEnumerationOracle(t *testing.T) {
	handler, st, username := newAuthServer(t)

	known := postJSON(t, handler, "/api/v1/auth/password-reset",
		map[string]string{"email": username + "@example.test"})
	unknown := postJSON(t, handler, "/api/v1/auth/password-reset",
		map[string]string{"email": "nobody@example.test"})

	if known.Code != http.StatusNoContent || unknown.Code != http.StatusNoContent {
		t.Fatalf("status: known=%d unknown=%d, want 204 for both", known.Code, unknown.Code)
	}
	if known.Body.String() != unknown.Body.String() {
		t.Fatalf("body differs:\nknown:   %q\nunknown: %q", known.Body.String(), unknown.Body.String())
	}

	// The intent is recorded for an administrator to act on.
	var count int
	if err := st.Pool().QueryRow(context.Background(),
		`SELECT count(*)::int FROM audit_log WHERE action = 'password_reset_request' AND target = $1`,
		username+"@example.test").Scan(&count); err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if count != 1 {
		t.Fatalf("audit rows = %d, want 1", count)
	}
}
