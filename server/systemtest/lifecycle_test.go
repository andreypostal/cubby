//go:build system

package systemtest

import (
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// The version has to be readable without a config file, because that is the
// state a freshly extracted archive or a bare container is in, and it is what
// the release pipeline smoke-tests the published artifact with.
func TestVersionCommandRunsWithoutAConfig(t *testing.T) {
	cmd := exec.Command(binaryPath, "version")
	// An empty directory, so nothing on disk can satisfy the config lookup
	cmd.Dir = t.TempDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running `cubby version`: %v\n%s", err, out)
	}

	if !strings.HasPrefix(string(out), "cubby ") {
		t.Errorf("`cubby version` printed %q", strings.TrimSpace(string(out)))
	}
}

// Every other command does need one, and saying so beats a stack trace.
func TestStartFailsWithoutAConfig(t *testing.T) {
	cmd := exec.Command(binaryPath, "start")
	cmd.Dir = t.TempDir()

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("`cubby start` succeeded without a config file:\n%s", out)
	}

	if !strings.Contains(string(out), "config") {
		t.Errorf("the failure does not mention the config file:\n%s", out)
	}
}

// A restart has to find the data that was there before. This is the only test
// that covers the database surviving the process, which is what separates a
// working deployment from one that greets every restart with the setup screen.
func TestDataSurvivesARestart(t *testing.T) {
	if !gracefulSignals {
		t.Skip("a graceful stop cannot be requested on this platform")
	}

	s := startServer(t)
	token := s.admin()
	s.workspace(token, "Home")

	var list struct {
		ID uint `json:"id"`
	}
	s.expect(http.MethodPost, "/api/v1/shop/lists",
		map[string]string{"name": "Groceries", "icon": "cart"}, token, http.StatusCreated, &list)

	// Same directory, same port: a restart of this server rather than a new one
	restarted := s.restart()

	var status struct {
		Required bool `json:"required"`
	}
	restarted.expect(http.MethodGet, "/api/v1/setup/status", nil, "", http.StatusOK, &status)
	if status.Required {
		t.Fatal("the restarted server asked to be set up again")
	}

	// The user outlived the process that created them, so the credentials are
	// still the way back in
	fresh := restarted.login("admin", "correct-horse-battery")

	var workspaces struct {
		Workspaces []struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		} `json:"workspaces"`
	}
	restarted.expect(http.MethodGet, "/api/v1/auth/workspaces", nil, fresh, http.StatusOK, &workspaces)
	if len(workspaces.Workspaces) != 1 || workspaces.Workspaces[0].Name != "Home" {
		t.Fatalf("the workspace did not survive the restart: %+v", workspaces.Workspaces)
	}

	restarted.expect(http.MethodPatch, "/api/v1/auth/me/workspace",
		map[string]uint{"workspace_id": workspaces.Workspaces[0].ID}, fresh, http.StatusNoContent, nil)

	var lists struct {
		Lists []struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		} `json:"lists"`
	}
	restarted.expect(http.MethodGet, "/api/v1/shop/lists", nil, fresh, http.StatusOK, &lists)

	if len(lists.Lists) != 1 || lists.Lists[0].Name != "Groceries" {
		t.Fatalf("the list did not survive the restart: %+v", lists.Lists)
	}
}

// SIGTERM is what a service manager and `docker stop` send, and the server has to
// treat it as a request to close its modules rather than dying where it stands.
func TestServerStopsGracefullyOnSignal(t *testing.T) {
	if !gracefulSignals {
		t.Skip("a graceful stop cannot be requested on this platform")
	}

	s := startServer(t)
	s.admin()

	if err := terminate(s.cmd); err != nil {
		t.Fatalf("signalling the server: %v", err)
	}

	select {
	case <-s.exited:
	case <-time.After(shutdownTimeout):
		t.Fatalf("the server was still running %s after SIGTERM\n%s", shutdownTimeout, s.output())
	}

	if s.waitErr != nil {
		t.Errorf("the server exited with %v\n%s", s.waitErr, s.output())
	}

	// The modules announce themselves as they close; a process that skipped that
	// stopped without flushing anything it was holding
	if out := s.output(); !strings.Contains(out, "closed") && !strings.Contains(out, "stopped") {
		t.Errorf("no module reported shutting down:\n%s", out)
	}
}
