//go:build system

package systemtest

import (
	"fmt"
	"net/http"
	"testing"
)

// Protected routes have to be closed to anyone without a session, checked here
// against the assembled binary rather than a middleware in isolation.
func TestProtectedRoutesRejectAnonymousCallers(t *testing.T) {
	s := startServer(t)
	s.admin()

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/auth/me"},
		{http.MethodGet, "/api/v1/auth/workspaces"},
		{http.MethodGet, "/api/v1/shop/lists"},
		{http.MethodGet, "/api/v1/users"},
	} {
		status, _ := s.do(route.method, route.path, nil, "")
		if status != http.StatusUnauthorized {
			t.Errorf("%s %s answered %d without a token, want 401", route.method, route.path, status)
		}

		status, _ = s.do(route.method, route.path, nil, "not-a-real-token")
		if status != http.StatusUnauthorized {
			t.Errorf("%s %s answered %d with a bogus token, want 401", route.method, route.path, status)
		}
	}
}

// Wrong credentials must not produce a session
func TestLoginRejectsBadCredentials(t *testing.T) {
	s := startServer(t)
	s.admin()

	status, _ := s.do(http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "not-the-password",
	}, "")

	if status != http.StatusUnauthorized && status != http.StatusForbidden {
		t.Fatalf("a login with the wrong password answered %d, want 401 or 403", status)
	}
}

// Workspace-scoped routes need an active workspace on the session, not just a
// logged-in user, so a fresh session cannot reach them until it selects one.
func TestWorkspaceScopeGatesTheShopRoutes(t *testing.T) {
	s := startServer(t)
	token := s.admin()

	status, _ := s.do(http.MethodGet, "/api/v1/shop/lists", nil, token)
	if status == http.StatusOK {
		t.Fatal("the shop routes answered a session with no active workspace")
	}

	s.workspace(token, "Home")

	var lists struct {
		Lists []struct {
			ID uint `json:"id"`
		} `json:"lists"`
	}
	s.expect(http.MethodGet, "/api/v1/shop/lists", nil, token, http.StatusOK, &lists)

	if len(lists.Lists) != 0 {
		t.Errorf("a new workspace already had %d lists", len(lists.Lists))
	}
}

// One user's path through the product, across the module boundaries the unit
// tests each cover from one side: log in, take a workspace, build a shopping
// list, tick something off, and read it back.
func TestShoppingListRoundTrip(t *testing.T) {
	s := startServer(t)
	token := s.admin()
	s.workspace(token, "Home")

	var list struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	s.expect(http.MethodPost, "/api/v1/shop/lists", map[string]string{
		"name": "Groceries",
		"icon": "cart",
	}, token, http.StatusCreated, &list)

	if list.ID == 0 {
		t.Fatal("the created list came back without an id")
	}

	var item struct {
		ID     uint   `json:"id"`
		Name   string `json:"name"`
		Bought bool   `json:"bought"`
	}
	itemsPath := fmt.Sprintf("/api/v1/shop/lists/%d/items", list.ID)
	s.expect(http.MethodPost, itemsPath, map[string]string{"name": "Coffee beans"},
		token, http.StatusCreated, &item)

	if item.Name != "Coffee beans" {
		t.Errorf("the created item is named %q, want Coffee beans", item.Name)
	}
	if item.Bought {
		t.Error("a newly created item is already marked bought")
	}

	var toggled struct {
		Bought bool `json:"bought"`
	}
	s.expect(http.MethodPatch, fmt.Sprintf("%s/%d/toggle", itemsPath, item.ID), nil,
		token, http.StatusOK, &toggled)

	if !toggled.Bought {
		t.Error("toggling the item did not mark it bought")
	}

	// Reading the list back is what proves the change was persisted rather than
	// only reflected in the response
	var stored struct {
		ID    uint `json:"id"`
		Items []struct {
			ID     uint   `json:"id"`
			Name   string `json:"name"`
			Bought bool   `json:"bought"`
		} `json:"items"`
	}
	s.expect(http.MethodGet, fmt.Sprintf("/api/v1/shop/lists/%d", list.ID), nil,
		token, http.StatusOK, &stored)

	if len(stored.Items) != 1 {
		t.Fatalf("the stored list holds %d items, want 1", len(stored.Items))
	}
	if !stored.Items[0].Bought {
		t.Error("the item was not persisted as bought")
	}
}

// A workspace is a boundary between users, and it has to hold when the request
// comes from a real session over the network.
func TestWorkspacesAreIsolatedBetweenUsers(t *testing.T) {
	s := startServer(t)
	admin := s.admin()
	adminWorkspace := s.workspace(admin, "Admin Home")

	s.expect(http.MethodPost, "/api/v1/users", map[string]any{
		"name":            "Outsider",
		"username":        "outsider",
		"password":        "correct-horse-battery",
		"is_system_admin": false,
	}, admin, http.StatusCreated, nil)

	outsider := s.login("outsider", "correct-horse-battery")

	// Somebody else's workspace is not theirs to select
	status, _ := s.do(http.MethodPatch, "/api/v1/auth/me/workspace",
		map[string]uint{"workspace_id": adminWorkspace}, outsider)
	if status != http.StatusForbidden {
		t.Errorf("switching into another user's workspace answered %d, want 403", status)
	}

	// Nor to read
	status, _ = s.do(http.MethodGet, fmt.Sprintf("/api/v1/auth/workspaces/%d", adminWorkspace), nil, outsider)
	if status != http.StatusForbidden && status != http.StatusNotFound {
		t.Errorf("reading another user's workspace answered %d, want 403 or 404", status)
	}

	// Their own workspace shows them an empty world rather than the admin's
	s.workspace(outsider, "Outsider Home")

	var lists struct {
		Lists []any `json:"lists"`
	}
	s.expect(http.MethodGet, "/api/v1/shop/lists", nil, outsider, http.StatusOK, &lists)
	if len(lists.Lists) != 0 {
		t.Errorf("a second user sees %d lists in their own workspace", len(lists.Lists))
	}
}

// A logout has to end the session server-side. A token that still works after it
// would mean the only way to revoke access is to wait for the expiry.
func TestLogoutInvalidatesTheSession(t *testing.T) {
	s := startServer(t)
	token := s.admin()

	s.expect(http.MethodGet, "/api/v1/auth/me", nil, token, http.StatusOK, nil)

	status, _ := s.do(http.MethodDelete, "/api/v1/auth/logout", nil, token)
	if status != http.StatusOK && status != http.StatusNoContent {
		t.Fatalf("logout answered %d", status)
	}

	if status, _ := s.do(http.MethodGet, "/api/v1/auth/me", nil, token); status != http.StatusUnauthorized {
		t.Errorf("the token still worked after logout, answering %d", status)
	}
}
