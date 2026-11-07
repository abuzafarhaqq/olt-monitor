package api_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/api"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/auth"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/crypto"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/domain"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/secrets"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/postgres"
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

// deviceServer builds the API with write endpoints and, optionally, credential
// storage (which requires a key).
func deviceServer(t *testing.T, withSecrets bool) (http.Handler, *postgres.Store, *secrets.Service) {
	t.Helper()

	st := openPostgres(t, testDSN(t))

	server := api.NewServer(st, nil).WithAuth(st).WithAudit(st).WithDevices(st).WithUsers(st)
	var service *secrets.Service
	if withSecrets {
		service = secrets.New(newKeyring(t), st)
		server = server.WithSecrets(service)
	}
	return server.Routes(), st, service
}

// loginAsUser creates the account and signs in, returning both. Tests that
// demote or delete administrators need the identity that owns the cookie —
// guessing "the first administrator" picks up accounts from other tests.
func loginAsUser(t *testing.T, handler http.Handler, st *postgres.Store, role string) (*http.Cookie, store.User) {
	t.Helper()

	username := fmt.Sprintf("u-%s-%d", role, time.Now().UnixNano())
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	user, err := st.CreateUser(context.Background(), store.CreateUserParams{
		Username: username, FullName: username, Email: username + "@example.test",
		Role: role, PasswordHash: hash,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	rec := postJSON(t, handler, "/api/v1/auth/login",
		map[string]string{"username": username, "password": "correct horse battery staple"})
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == api.SessionCookieName {
			return cookie, user
		}
	}
	t.Fatalf("login failed for %s: %d %s", role, rec.Code, rec.Body.String())
	return nil, store.User{}
}

func loginAs(t *testing.T, handler http.Handler, st *postgres.Store, role string) *http.Cookie {
	t.Helper()
	cookie, _ := loginAsUser(t, handler, st, role)
	return cookie
}

func call(t *testing.T, handler http.Handler, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	payload := []byte{}
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		payload = encoded
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func validDevice(name string) map[string]any {
	return map[string]any{
		"name":              name,
		"vendor":            "VSOL",
		"model":             "GPON-OLT",
		"technology":        "GPON",
		"managementAddress": "192.0.2.77",
		"connection":        "ssh",
	}
}

func TestCreateOltNeedsAuthenticationAndPermission(t *testing.T) {
	handler, st, _ := deviceServer(t, true)

	if rec := call(t, handler, http.MethodPost, "/api/v1/olts", validDevice("anon"), nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous create = %d, want 401", rec.Code)
	}

	viewer := loginAs(t, handler, st, "viewer")
	if rec := call(t, handler, http.MethodPost, "/api/v1/olts", validDevice("viewer-made"), viewer); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer create = %d, want 403", rec.Code)
	}

	admin := loginAs(t, handler, st, "administrator")
	rec := call(t, handler, http.MethodPost, "/api/v1/olts", validDevice("admin-made"), admin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin create = %d, body=%s", rec.Code, rec.Body.String())
	}

	var created domain.Olt
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == "" || created.Name != "admin-made" {
		t.Fatalf("unexpected device: %+v", created)
	}
	if created.CollectionStatus != domain.CollectionFailed {
		// A device that has never been collected must not look healthy.
		t.Fatalf("new device collection status = %q, want failed", created.CollectionStatus)
	}
}

func TestCreateOltValidatesInput(t *testing.T) {
	handler, st, _ := deviceServer(t, true)
	admin := loginAs(t, handler, st, "administrator")

	cases := map[string]map[string]any{
		"no name":       {"name": "", "vendor": "VSOL", "model": "X", "technology": "GPON", "managementAddress": "192.0.2.1", "connection": "ssh"},
		"bad ip":        {"name": "a", "vendor": "VSOL", "model": "X", "technology": "GPON", "managementAddress": "not-an-ip", "connection": "ssh"},
		"unknown tech":  {"name": "b", "vendor": "VSOL", "model": "X", "technology": "XPON", "managementAddress": "192.0.2.1", "connection": "ssh"},
		"bad transport": {"name": "c", "vendor": "VSOL", "model": "X", "technology": "GPON", "managementAddress": "192.0.2.1", "connection": "ftp"},
	}
	for name, body := range cases {
		if rec := call(t, handler, http.MethodPost, "/api/v1/olts", body, admin); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rec.Code)
		}
	}

	// Lower-case input is normalised, not rejected: "epon" would otherwise miss
	// every threshold lookup that keys on "EPON".
	lower := validDevice("lower-case-tech")
	lower["technology"] = "epon"
	rec := call(t, handler, http.MethodPost, "/api/v1/olts", lower, admin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("lower-case technology = %d, want 201", rec.Code)
	}
	var created domain.Olt
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Technology != "EPON" {
		t.Fatalf("stored technology = %q, want EPON", created.Technology)
	}
}

// The credential is write-only: it can be set, its metadata can be read, and
// nothing anywhere returns the secret.
func TestCredentialIsWriteOnlyAndEncrypted(t *testing.T) {
	handler, st, service := deviceServer(t, true)
	admin := loginAs(t, handler, st, "administrator")

	create := call(t, handler, http.MethodPost, "/api/v1/olts", validDevice("cred-target"), admin)
	var device domain.Olt
	if err := json.Unmarshal(create.Body.Bytes(), &device); err != nil {
		t.Fatalf("decode: %v", err)
	}

	const password = "device-password-9f3a"
	set := call(t, handler, http.MethodPut, "/api/v1/olts/"+device.ID+"/credential",
		map[string]string{"username": "admin", "password": password}, admin)
	if set.Code != http.StatusNoContent {
		t.Fatalf("set credential = %d, body=%s", set.Code, set.Body.String())
	}
	if bytes.Contains(set.Body.Bytes(), []byte(password)) {
		t.Fatal("the set-credential response echoes the password")
	}

	meta := call(t, handler, http.MethodGet, "/api/v1/olts/"+device.ID+"/credential", nil, admin)
	if meta.Code != http.StatusOK {
		t.Fatalf("get meta = %d", meta.Code)
	}
	if bytes.Contains(meta.Body.Bytes(), []byte(password)) {
		t.Fatalf("credential metadata leaked the secret: %s", meta.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(meta.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode meta: %v", err)
	}
	if body["username"] != "admin" {
		t.Fatalf("meta username = %v", body["username"])
	}
	if _, present := body["password"]; present {
		t.Fatal("metadata contains a password field")
	}

	var sealed []byte
	if err := st.Pool().QueryRow(context.Background(),
		`SELECT secret_enc FROM olt_credentials WHERE olt_id = $1`, device.ID).Scan(&sealed); err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	if bytes.Contains(sealed, []byte(password)) {
		t.Fatal("the stored blob contains the plaintext password")
	}

	username, resolved, err := service.Resolve(context.Background(), device.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if username != "admin" || resolved != password {
		t.Fatalf("resolve returned %q / %q", username, resolved)
	}
}

func TestCredentialRequiresASecretStore(t *testing.T) {
	handler, st, _ := deviceServer(t, false)
	admin := loginAs(t, handler, st, "administrator")

	create := call(t, handler, http.MethodPost, "/api/v1/olts", validDevice("no-secrets"), admin)
	var device domain.Olt
	if err := json.Unmarshal(create.Body.Bytes(), &device); err != nil {
		t.Fatalf("decode: %v", err)
	}

	rec := call(t, handler, http.MethodPut, "/api/v1/olts/"+device.ID+"/credential",
		map[string]string{"username": "admin", "password": "x"}, admin)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when no key is configured", rec.Code)
	}
}

func TestViewerCanReadCredentialMetadataButNotWrite(t *testing.T) {
	handler, st, _ := deviceServer(t, true)
	admin := loginAs(t, handler, st, "administrator")
	viewer := loginAs(t, handler, st, "viewer")

	create := call(t, handler, http.MethodPost, "/api/v1/olts", validDevice("read-only-meta"), admin)
	var device domain.Olt
	if err := json.Unmarshal(create.Body.Bytes(), &device); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if rec := call(t, handler, http.MethodPut, "/api/v1/olts/"+device.ID+"/credential",
		map[string]string{"username": "admin", "password": "x"}, viewer); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer write = %d, want 403", rec.Code)
	}
	if rec := call(t, handler, http.MethodGet, "/api/v1/olts/"+device.ID+"/credential", nil, viewer); rec.Code != http.StatusNotFound {
		t.Fatalf("viewer meta read = %d, want 404 (nothing set yet)", rec.Code)
	}
}
