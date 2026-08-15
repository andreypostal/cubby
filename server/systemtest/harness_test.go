//go:build system

// Package systemtest drives a real cubby binary over real HTTP.
//
// Everything else in the tree tests packages in isolation. These tests test the
// artifact: the same binary the release pipeline ships, started as a user starts
// it, talked to over a socket. That makes them the only place where a mistake in
// how the binary is assembled shows up — most importantly a build that forgot to
// produce the web UI, which no unit test can notice because the embedded
// filesystem is a build input, not code.
//
// They are behind the `system` build tag so `go test ./...` stays fast and free
// of the compile-and-boot cost. Run them with:
//
//	go test -tags system ./server/systemtest/...
//
// The UI has to exist before the binary is compiled, so `make generate` (or a
// prior `npm --prefix web run build`) is a precondition; the harness fails with
// that instruction rather than a confusing boot error.
package systemtest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// How long the server gets to bind its port and answer the first request
	startupTimeout = 60 * time.Second
	// How long a graceful shutdown may take before the process is killed
	shutdownTimeout = 30 * time.Second
)

// binaryPath is the server under test, resolved once for the whole package.
var binaryPath string

// setupTokenRe matches the attribute the bootstrap module logs on a server that
// has no users yet, in either encoding: the text handler writes
// `setup_token=abc` and the json one `"setup_token":"abc"`. The harness runs
// with env prod, where the format defaults to json, but pinning the test to one
// handler would mean a format change breaking every test in the package.
var setupTokenRe = regexp.MustCompile(`setup_token"?[=:]"?([0-9a-f]+)`)

func TestMain(m *testing.M) {
	// Letting CI point at an already-built artifact is the whole reason this is
	// configurable: the pipeline tests the binary it is about to publish, rather
	// than a second one compiled with different flags.
	if prebuilt := os.Getenv("CUBBY_SYSTEM_TEST_BINARY"); prebuilt != "" {
		resolved, err := resolveBinary(prebuilt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		binaryPath = resolved
	} else {
		tmp, err := os.MkdirTemp("", "cubby-systemtest-")
		if err != nil {
			fmt.Fprintf(os.Stderr, "creating the build directory: %v\n", err)
			os.Exit(1)
		}
		defer os.RemoveAll(tmp)

		binaryPath = filepath.Join(tmp, "cubby"+exeSuffix())
		if err := buildServer(binaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}

	os.Exit(m.Run())
}

func exeSuffix() string {
	if os.PathSeparator == '\\' {
		return ".exe"
	}
	return ""
}

// resolveBinary turns the configured path into one that can actually be executed.
// A relative path is taken as relative to the repository root, because that is
// where it was typed: `go test` runs the binary with its working directory set to
// this package, so resolving against the process's own directory would look for
// the server underneath server/systemtest.
func resolveBinary(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	root, err := repoRoot()
	if err != nil {
		return "", fmt.Errorf("resolving CUBBY_SYSTEM_TEST_BINARY %q: %w", path, err)
	}

	resolved := filepath.Join(root, path)
	if _, err := os.Stat(resolved); err != nil {
		return "", fmt.Errorf("CUBBY_SYSTEM_TEST_BINARY %q does not exist at %s: %w", path, resolved, err)
	}

	return resolved, nil
}

// buildServer compiles the server the way a developer would, from the module
// root, so the embedded UI in the working tree is what gets tested.
func buildServer(out string) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

	cmd := exec.Command("go", "build", "-o", out, "./server/cmd/server")
	cmd.Dir = root

	if combined, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("building the server: %w\n%s", err, combined)
	}

	return nil
}

// repoRoot walks up from the test's working directory to the directory holding
// go.mod, which is where the build and the npm prefix paths are anchored.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locating the working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above %q", dir)
		}
		dir = parent
	}
}

// server is a running cubby process plus everything a test needs to talk to it
type server struct {
	t *testing.T

	// dir and port are kept so a restart can come back as the same deployment
	dir  string
	port int

	baseURL string
	cmd     *exec.Cmd
	client  *http.Client

	// logs accumulates the process output, which is where the setup token and
	// any startup failure show up
	mu   sync.Mutex
	logs bytes.Buffer

	// exited is closed once the process has been reaped
	exited  chan struct{}
	waitErr error
}

// startServer boots a server on its own port against an empty data directory and
// stops it when the test ends.
func startServer(t *testing.T) *server {
	t.Helper()

	dir := t.TempDir()
	port := freePort(t)

	writeConfig(t, filepath.Join(dir, "config.yaml"), port)

	return launch(t, dir, port)
}

// restart stops the server and brings a new process up on the same directory and
// port, which is how a deployment gets upgraded or rebooted.
func (s *server) restart() *server {
	s.t.Helper()

	s.stop()

	return launch(s.t, s.dir, s.port)
}

// launch runs one server process against an already prepared directory
func launch(t *testing.T, dir string, port int) *server {
	t.Helper()

	s := &server{
		t:       t,
		dir:     dir,
		port:    port,
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		client: &http.Client{
			Timeout: 30 * time.Second,
			// Every assertion here is about the status and body of one
			// response, and a redirect would silently turn a 3xx into
			// somebody else's 200.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		exited: make(chan struct{}),
	}

	cmd := exec.Command(binaryPath, "start")
	cmd.Dir = dir
	// A clean environment keeps a developer's exported CUBBY_* variables from
	// reaching into the test, but PATH still has to survive for the OS to load
	// the process.
	cmd.Env = append(os.Environ(), "CUBBY_ENV=prod")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting %s: %v", binaryPath, err)
	}
	s.cmd = cmd

	var streams sync.WaitGroup
	streams.Add(2)
	go s.drain(&streams, stdout)
	go s.drain(&streams, stderr)

	go func() {
		// Waiting only after both pipes hit EOF, since Wait closes them
		streams.Wait()
		s.waitErr = cmd.Wait()
		close(s.exited)
	}()

	t.Cleanup(s.stop)

	s.waitReady()

	return s
}

// drain copies one of the process streams into the log buffer as it arrives, so
// the setup token is readable while the server is still running.
func (s *server) drain(wg *sync.WaitGroup, r io.Reader) {
	defer wg.Done()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		s.mu.Lock()
		s.logs.WriteString(scanner.Text())
		s.logs.WriteByte('\n')
		s.mu.Unlock()
	}
}

func (s *server) output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logs.String()
}

// waitReady blocks until the server answers, or fails the test with the process
// output, which is where a refusal to start explains itself.
func (s *server) waitReady() {
	s.t.Helper()

	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-s.exited:
			s.t.Fatalf("the server exited during startup (%v)\n%s", s.waitErr, s.output())
		default:
		}

		res, err := s.client.Get(s.baseURL + "/api/v1/setup/status")
		if err == nil {
			res.Body.Close()
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	s.t.Fatalf("the server did not answer within %s\n%s", startupTimeout, s.output())
}

// stop asks the process to shut down the way a service manager would and gives
// it a bounded time to finish before killing it.
func (s *server) stop() {
	select {
	case <-s.exited:
		return
	default:
	}

	// Windows has no SIGTERM to deliver, so the only portable way to end the
	// process there is to kill it. The graceful path gets its own test.
	_ = terminate(s.cmd)

	select {
	case <-s.exited:
	case <-time.After(shutdownTimeout):
		_ = s.cmd.Process.Kill()
		<-s.exited
	}
}

// waitForOutput polls what the server has written until match accepts it,
// reporting whether it turned up. The streams are drained on their own
// goroutines, so a line is not on hand the moment the request it describes
// comes back.
func (s *server) waitForOutput(match func(string) bool) bool {
	s.t.Helper()

	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		if match(s.output()) {
			return true
		}

		time.Sleep(50 * time.Millisecond)
	}

	return false
}

// setupToken pulls the bootstrap token out of the server's log, waiting for the
// line to appear since it is written concurrently with the port opening.
func (s *server) setupToken() string {
	s.t.Helper()

	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		if m := setupTokenRe.FindStringSubmatch(s.output()); m != nil {
			return m[1]
		}
		time.Sleep(50 * time.Millisecond)
	}

	s.t.Fatalf("no setup token was logged\n%s", s.output())
	return ""
}

// do sends a request, optionally authenticated, and returns the status and body.
// Bodies here are small JSON documents, so reading them whole keeps the call
// sites to one line.
func (s *server) do(method, path string, body any, token string) (int, []byte) {
	s.t.Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			s.t.Fatalf("encoding the %s %s body: %v", method, path, err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, s.baseURL+path, reader)
	if err != nil {
		s.t.Fatalf("building the %s %s request: %v", method, path, err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := s.client.Do(req)
	if err != nil {
		s.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()

	payload, err := io.ReadAll(res.Body)
	if err != nil {
		s.t.Fatalf("reading the %s %s response: %v", method, path, err)
	}

	return res.StatusCode, payload
}

// expect runs a request that is meant to succeed and decodes it into out, which
// may be nil for responses with no body worth reading.
func (s *server) expect(method, path string, body any, token string, want int, out any) {
	s.t.Helper()

	status, payload := s.do(method, path, body, token)
	if status != want {
		s.t.Fatalf("%s %s answered %d, want %d: %s", method, path, status, want, payload)
	}

	if out == nil {
		return
	}

	if err := json.Unmarshal(payload, out); err != nil {
		s.t.Fatalf("decoding the %s %s response: %v: %s", method, path, err, payload)
	}
}

// get issues a raw GET and hands back the whole response, for the tests that care
// about headers rather than a decoded body.
func (s *server) get(path string, headers map[string]string) (*http.Response, []byte) {
	s.t.Helper()

	req, err := http.NewRequest(http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		s.t.Fatalf("building the GET %s request: %v", path, err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := s.client.Do(req)
	if err != nil {
		s.t.Fatalf("GET %s: %v", path, err)
	}
	defer res.Body.Close()

	payload, err := io.ReadAll(res.Body)
	if err != nil {
		s.t.Fatalf("reading the GET %s response: %v", path, err)
	}

	return res, payload
}

// admin completes the first-run setup and logs in, returning a bearer token.
// Most tests need a usable session rather than the bootstrap itself.
func (s *server) admin() string {
	s.t.Helper()

	s.expect(http.MethodPost, "/api/v1/setup", map[string]string{
		"token":    s.setupToken(),
		"name":     "Test Admin",
		"username": "admin",
		"password": "correct-horse-battery",
	}, "", http.StatusNoContent, nil)

	return s.login("admin", "correct-horse-battery")
}

func (s *server) login(username, password string) string {
	s.t.Helper()

	var out struct {
		Token string `json:"token"`
	}

	s.expect(http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, "", http.StatusOK, &out)

	if out.Token == "" {
		s.t.Fatal("login returned an empty token")
	}

	return out.Token
}

// workspace creates a workspace and makes it the session's active one, which is
// what the workspace-scoped routes require.
func (s *server) workspace(token, name string) uint {
	s.t.Helper()

	var created struct {
		ID uint `json:"id"`
	}

	s.expect(http.MethodPost, "/api/v1/auth/workspaces",
		map[string]string{"name": name}, token, http.StatusCreated, &created)

	s.expect(http.MethodPatch, "/api/v1/auth/me/workspace",
		map[string]uint{"workspace_id": created.ID}, token, http.StatusNoContent, nil)

	return created.ID
}

// freePort asks the kernel for an unused port and releases it. The server binds
// it moments later; nothing else on a CI runner is racing for it.
func freePort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	defer ln.Close()

	return ln.Addr().(*net.TCPAddr).Port
}

// writeConfig lays down the minimum the server insists on. Paths stay relative
// so everything the run creates lands inside the test's temporary directory.
func writeConfig(t *testing.T, path string, port int) {
	t.Helper()

	config := strings.ReplaceAll(`env: prod
data_dir: data
backup_dir: backups
backup_keep: 2

api:
  host: 127.0.0.1
  port: PORT
  read_timeout: 10s
  write_timeout: 60s
  idle_timeout: 60s
  cors:
    allowed_origins:
      - "*"
    allowed_methods: [GET, POST, PUT, PATCH, DELETE, OPTIONS]
    allowed_headers: [Origin, Content-Type, Accept, Authorization]
    exposed_headers: [Content-Disposition]
    max_age: 3600

db:
  sqlite_params: "_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)"
  max_open_conns: 4
  max_idle_conns: 2
  conn_max_lifetime: 1h

jwt:
  expires_in: 6h

indexer:
  writer_pool_size: 2
  reader_pool_size: 2
  writer_idle_time: 5m
  reader_idle_time: 10m
  suggestion_limit: 5
  suggest_timeout: 5s

web:
  enabled: true
`, "PORT", fmt.Sprint(port))

	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatalf("writing the config: %v", err)
	}
}
