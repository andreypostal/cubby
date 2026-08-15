//go:build system

package systemtest

import (
	"net/http"
	"testing"
)

// A server with no users announces that fact and hands out a token, which is the
// only credential it can offer before an administrator exists.
func TestFirstRunRequiresSetup(t *testing.T) {
	s := startServer(t)

	var status struct {
		Required bool `json:"required"`
	}
	s.expect(http.MethodGet, "/api/v1/setup/status", nil, "", http.StatusOK, &status)

	if !status.Required {
		t.Fatal("a fresh server reported that no setup is required")
	}

	if token := s.setupToken(); len(token) != 48 {
		t.Errorf("the logged setup token is %d characters, want 48", len(token))
	}
}

// The token is what stops anybody who can reach the port from claiming the
// server, so a wrong one has to be refused and must not consume the real one.
func TestSetupRejectsAWrongToken(t *testing.T) {
	s := startServer(t)
	s.setupToken()

	status, _ := s.do(http.MethodPost, "/api/v1/setup", map[string]string{
		"token":    "0000000000000000000000000000000000000000000000ff",
		"name":     "Intruder",
		"username": "intruder",
		"password": "correct-horse-battery",
	}, "")

	if status != http.StatusForbidden {
		t.Fatalf("setup with a wrong token answered %d, want 403", status)
	}

	// The real token still has to work afterwards
	var check struct {
		Required bool `json:"required"`
	}
	s.expect(http.MethodGet, "/api/v1/setup/status", nil, "", http.StatusOK, &check)
	if !check.Required {
		t.Fatal("a rejected attempt cleared the setup requirement")
	}
}

// The full first-run path: claim the server, then find it closed to a second
// claim and open to a login.
func TestSetupCompletesAndClosesItself(t *testing.T) {
	s := startServer(t)
	token := s.setupToken()

	s.expect(http.MethodPost, "/api/v1/setup", map[string]string{
		"token":    token,
		"name":     "Test Admin",
		"username": "admin",
		"password": "correct-horse-battery",
	}, "", http.StatusNoContent, nil)

	var status struct {
		Required bool `json:"required"`
	}
	s.expect(http.MethodGet, "/api/v1/setup/status", nil, "", http.StatusOK, &status)
	if status.Required {
		t.Fatal("the server still reports setup as required after completing it")
	}

	// Replaying the same request must not create a second administrator
	replay, _ := s.do(http.MethodPost, "/api/v1/setup", map[string]string{
		"token":    token,
		"name":     "Second Admin",
		"username": "second",
		"password": "correct-horse-battery",
	}, "")
	if replay != http.StatusConflict {
		t.Errorf("a replayed setup answered %d, want 409", replay)
	}

	var me struct {
		User struct {
			Username      string `json:"username"`
			IsSystemAdmin bool   `json:"is_system_admin"`
		} `json:"user"`
	}
	s.expect(http.MethodGet, "/api/v1/auth/me", nil,
		s.login("admin", "correct-horse-battery"), http.StatusOK, &me)

	if me.User.Username != "admin" {
		t.Errorf("logged in as %q, want admin", me.User.Username)
	}
	// Nobody exists to promote the first user, so setup has to do it
	if !me.User.IsSystemAdmin {
		t.Error("the first user was not made a system administrator")
	}
}

// Validation runs before the token is checked against the request, and a weak
// password must not slip through the one endpoint that has no session behind it.
func TestSetupValidatesItsPayload(t *testing.T) {
	s := startServer(t)
	token := s.setupToken()

	status, _ := s.do(http.MethodPost, "/api/v1/setup", map[string]string{
		"token":    token,
		"name":     "Test Admin",
		"username": "admin",
		"password": "short",
	}, "")

	if status != http.StatusBadRequest && status != http.StatusUnprocessableEntity {
		t.Fatalf("setup with a 5 character password answered %d, want a validation failure", status)
	}

	// A rejected payload must leave the server claimable
	var check struct {
		Required bool `json:"required"`
	}
	s.expect(http.MethodGet, "/api/v1/setup/status", nil, "", http.StatusOK, &check)
	if !check.Required {
		t.Fatal("a rejected payload cleared the setup requirement")
	}
}
