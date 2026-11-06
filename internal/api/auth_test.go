package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/abuzafarhaqq/olt-monitor-backend/internal/api"
	"github.com/abuzafarhaqq/olt-monitor-backend/internal/auth"
	"github.com/abuzafarhaqq/olt-monitor-backend/internal/store/postgres"
	"github.com/abuzafarhaqq/olt-monitor-backend/migrations"
)

// Login needs a database: the in-memory fixture has no users, and inventing a
// fake user store would test the fake instead of the real path.
func newAuthServer(t *testing.T) (http.Handler, *postgres.Store, string) {
	t.Helper()

	dsn := testDSN(t)
	st := openPostgres(t, dsn)

	username := fmt.Sprintf("tester-%d", time.Now().UnixNano())
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if _, err := st.CreateUser(context.Background(), postgres.CreateUserParams{
		Username:     username,
		FullName:     "NOC Operator",
		Email:        username + "@example.test",
		Role:         "noc-operator",
		PasswordHash: hash,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	return api.NewServer(st, nil).WithAuth(st).Routes(), st, username
}

func postJSON(t *testing.T, handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	handler, _, username := newAuthServer(t)

	rec := postJSON(t, handler, "/api/v1/auth/login",
		map[string]string{"username": username, "password": "wrong password"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == api.SessionCookieName && cookie.Value != "" {
			t.Fatal("a session cookie was issued for a failed login")
		}
	}
}

// An unknown user and a wrong password must be indistinguishable.
func TestLoginDoesNotRevealWhichUsernameExists(t *testing.T) {
	handler, _, username := newAuthServer(t)

	known := postJSON(t, handler, "/api/v1/auth/login",
		map[string]string{"username": username, "password": "nope-nope-nope"})
	unknown := postJSON(t, handler, "/api/v1/auth/login",
		map[string]string{"username": "nobody-here", "password": "nope-nope-nope"})

	if known.Code != unknown.Code {
		t.Fatalf("status differs: known=%d unknown=%d", known.Code, unknown.Code)
	}
	if known.Body.String() != unknown.Body.String() {
		t.Fatalf("body differs:\nknown:   %s\nunknown: %s", known.Body.String(), unknown.Body.String())
	}
}

func TestLoginMeLogout(t *testing.T) {
	handler, st, username := newAuthServer(t)

	login := postJSON(t, handler, "/api/v1/auth/login",
		map[string]string{"username": username, "password": "correct horse battery staple"})
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body=%s", login.Code, login.Body.String())
	}

	var cookie *http.Cookie
	for _, c := range login.Result().Cookies() {
		if c.Name == api.SessionCookieName {
			cookie = c
		}
	}
	if cookie == nil || cookie.Value == "" {
		t.Fatal("no session cookie")
	}
	if !cookie.HttpOnly {
		t.Fatal("cookie is not HttpOnly: JavaScript could read the session")
	}

	// The response must not carry the hash or any secret material.
	if bytes.Contains(login.Body.Bytes(), []byte("argon2")) {
		t.Fatalf("login response leaked a hash: %s", login.Body.String())
	}

	me := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := me(); rec.Code != http.StatusOK {
		t.Fatalf("me status = %d", rec.Code)
	}

	// The cookie value is not in the database: only its hash is.
	user, err := st.UserBySession(context.Background(), auth.HashToken(cookie.Value))
	if err != nil {
		t.Fatalf("session lookup: %v", err)
	}
	if user.Username != username {
		t.Fatalf("session belongs to %q, want %q", user.Username, username)
	}

	logout := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logout.AddCookie(cookie)
	logoutRec := httptest.NewRecorder()
	handler.ServeHTTP(logoutRec, logout)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logoutRec.Code)
	}

	if rec := me(); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout = %d, want 401", rec.Code)
	}
}

func TestMeWithoutCookieIsUnauthorized(t *testing.T) {
	handler, _, _ := newAuthServer(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestDisabledUserCannotLogIn(t *testing.T) {
	handler, st, username := newAuthServer(t)

	user, err := st.UserByUsername(context.Background(), username)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if _, err := st.Pool().Exec(context.Background(),
		`UPDATE users SET disabled = TRUE WHERE id = $1`, user.ID); err != nil {
		t.Fatalf("disable: %v", err)
	}

	rec := postJSON(t, handler, "/api/v1/auth/login",
		map[string]string{"username": username, "password": "correct horse battery staple"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("disabled user login = %d, want 401", rec.Code)
	}
}

// Guards against a real bug: a session row that outlives its expiry must stop
// authenticating, even though nothing deleted it.
func TestExpiredSessionIsRejected(t *testing.T) {
	handler, st, username := newAuthServer(t)

	login := postJSON(t, handler, "/api/v1/auth/login",
		map[string]string{"username": username, "password": "correct horse battery staple"})
	var cookie *http.Cookie
	for _, c := range login.Result().Cookies() {
		if c.Name == api.SessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no cookie")
	}

	if _, err := st.Pool().Exec(context.Background(),
		`UPDATE sessions SET expires_at = now() - interval '1 minute' WHERE token_hash = $1`,
		auth.HashToken(cookie.Value)); err != nil {
		t.Fatalf("expire: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired session = %d, want 401", rec.Code)
	}
}

func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping authentication tests that need a database")
	}
	return dsn
}

func openPostgres(t *testing.T, dsn string) *postgres.Store {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	st, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(st.Close)
	if err := postgres.Migrate(ctx, st.Pool(), migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}
