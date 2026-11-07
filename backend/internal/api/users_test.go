package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/postgres"
)

func newUser(name string) map[string]any {
	return map[string]any{
		"username": name, "fullName": "Test " + name, "email": name + "@example.test",
		"role": "viewer", "password": "correct horse battery staple",
	}
}

func TestUserLifecycle(t *testing.T) {
	handler, st, _ := deviceServer(t, true)
	admin := loginAs(t, handler, st, "administrator")

	created := call(t, handler, http.MethodPost, "/api/v1/users", newUser("clerk"), admin)
	if created.Code != http.StatusCreated {
		t.Fatalf("create = %d, body=%s", created.Code, created.Body.String())
	}
	var record map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &record); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, leaked := record["password"]; leaked {
		t.Fatal("the response contains a password field")
	}
	if _, leaked := record["passwordHash"]; leaked {
		t.Fatal("the response contains a hash")
	}
	id := record["id"].(string)

	// Promote to administrator, then confirm the change is persisted.
	patch := map[string]any{
		"fullName": "Test clerk", "email": "clerk@example.test",
		"role": "administrator", "disabled": false,
	}
	updated := call(t, handler, http.MethodPatch, "/api/v1/users/"+id, patch, admin)
	if updated.Code != http.StatusOK {
		t.Fatalf("patch = %d, body=%s", updated.Code, updated.Body.String())
	}

	// Password reset must revoke that user's sessions.
	reset := call(t, handler, http.MethodPut, "/api/v1/users/"+id+"/password",
		map[string]string{"password": "another-long-password"}, admin)
	if reset.Code != http.StatusNoContent {
		t.Fatalf("password reset = %d", reset.Code)
	}

	removed := call(t, handler, http.MethodDelete, "/api/v1/users/"+id, nil, admin)
	if removed.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, body=%s", removed.Code, removed.Body.String())
	}
	if _, err := st.UserByID(context.Background(), id); err == nil {
		t.Fatal("the account still exists after deletion")
	}
}

// The system must never end up with zero administrators: that is unrecoverable
// without shell access.
func TestLastAdministratorCannotBeRemoved(t *testing.T) {
	handler, st, _ := deviceServer(t, true)
	admin, me := loginAsUser(t, handler, st, "administrator")

	// Every other administrator becomes a viewer, so `me` is the last one.
	if _, err := st.Pool().Exec(context.Background(),
		`UPDATE users SET role = 'viewer' WHERE role = 'administrator' AND id <> $1`, me.ID); err != nil {
		t.Fatalf("demote others: %v", err)
	}

	demote := call(t, handler, http.MethodPatch, "/api/v1/users/"+me.ID,
		map[string]any{"fullName": me.FullName, "email": me.Email, "role": "viewer"}, admin)
	if demote.Code != http.StatusConflict {
		t.Fatalf("demoting the last admin = %d, want 409 (%s)", demote.Code, demote.Body.String())
	}

	del := call(t, handler, http.MethodDelete, "/api/v1/users/"+me.ID, nil, admin)
	if del.Code != http.StatusConflict {
		t.Fatalf("deleting yourself = %d, want 409", del.Code)
	}
}

// Deleting an administrator is allowed while another one remains.
func TestAdministratorCanBeDeletedWhenAnotherRemains(t *testing.T) {
	handler, st, _ := deviceServer(t, true)
	admin, _ := loginAsUser(t, handler, st, "administrator")

	second := call(t, handler, http.MethodPost, "/api/v1/users", newUser("second-admin"), admin)
	if second.Code != http.StatusCreated {
		t.Fatalf("create = %d, body=%s", second.Code, second.Body.String())
	}
	var record map[string]any
	if err := json.Unmarshal(second.Body.Bytes(), &record); err != nil {
		t.Fatalf("decode: %v", err)
	}
	id := record["id"].(string)

	if rec := call(t, handler, http.MethodPatch, "/api/v1/users/"+id,
		map[string]any{"fullName": "Second", "email": "second@example.test", "role": "administrator"}, admin); rec.Code != http.StatusOK {
		t.Fatalf("promote = %d", rec.Code)
	}

	// Two active administrators now: demoting one is fine.
	if rec := call(t, handler, http.MethodPatch, "/api/v1/users/"+id,
		map[string]any{"fullName": "Second", "email": "second@example.test", "role": "viewer"}, admin); rec.Code != http.StatusOK {
		t.Fatalf("demote with a peer remaining = %d, want 200", rec.Code)
	}
}

func TestUserEndpointsNeedPermission(t *testing.T) {
	handler, st, _ := deviceServer(t, true)
	viewer := loginAs(t, handler, st, "viewer")

	if rec := call(t, handler, http.MethodGet, "/api/v1/users", nil, viewer); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer list = %d, want 403", rec.Code)
	}
	if rec := call(t, handler, http.MethodPost, "/api/v1/users", newUser("nope"), viewer); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer create = %d, want 403", rec.Code)
	}
	if rec := call(t, handler, http.MethodPost, "/api/v1/users", newUser("anon"), nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous create = %d, want 401", rec.Code)
	}
}

func TestDeviceDeleteRequiresPermission(t *testing.T) {
	handler, st, _ := deviceServer(t, true)
	admin := loginAs(t, handler, st, "administrator")
	viewer := loginAs(t, handler, st, "viewer")

	create := call(t, handler, http.MethodPost, "/api/v1/olts", validDevice("delete-me"), admin)
	var device map[string]any
	if err := json.Unmarshal(create.Body.Bytes(), &device); err != nil {
		t.Fatalf("decode: %v", err)
	}
	id := device["id"].(string)

	if rec := call(t, handler, http.MethodDelete, "/api/v1/olts/"+id, nil, viewer); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer delete = %d, want 403", rec.Code)
	}
	if rec := call(t, handler, http.MethodDelete, "/api/v1/olts/"+id, nil, admin); rec.Code != http.StatusNoContent {
		t.Fatalf("admin delete = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec := call(t, handler, http.MethodDelete, "/api/v1/olts/"+id, nil, admin); rec.Code != http.StatusNotFound {
		t.Fatalf("second delete = %d, want 404", rec.Code)
	}
}

func lastAdminUsername(t *testing.T, st *postgres.Store) string {
	t.Helper()
	rows, err := st.Pool().Query(context.Background(),
		`SELECT username FROM users WHERE role = 'administrator' AND disabled = FALSE`)
	if err != nil {
		t.Fatalf("query admins: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if bytes.HasPrefix([]byte(name), []byte("u-administrator-")) {
			return name
		}
	}
	t.Fatal("no administrator account created by the test")
	return ""
}
