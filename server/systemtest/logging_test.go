//go:build system

package systemtest

import (
	"net/http"
	"strings"
	"testing"
)

// The id is only worth having if a caller can quote it, so it has to come back
// on the response and turn up in what the server wrote about that request.
func TestARequestIsCorrelatableFromItsResponseHeader(t *testing.T) {
	s := startServer(t)

	res, err := s.client.Get(s.baseURL + "/api/v1/setup/status")
	if err != nil {
		t.Fatalf("GET /api/v1/setup/status: %v", err)
	}
	defer res.Body.Close()

	id := res.Header.Get("X-Request-Id")
	if id == "" {
		t.Fatal("the response carries no X-Request-Id header")
	}

	if !s.waitForOutput(func(out string) bool { return strings.Contains(out, id) }) {
		t.Errorf("the id %q from the response is in nothing the server logged:\n%s", id, s.output())
	}
}

// A reverse proxy or a caller that already tracks an id keeps it across the hop,
// which is what makes the server's lines line up with whatever sits in front.
func TestAnIdSentByTheCallerSurvivesTheRoundTrip(t *testing.T) {
	s := startServer(t)

	req, err := http.NewRequest(http.MethodGet, s.baseURL+"/api/v1/setup/status", nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("X-Request-Id", "systemtest-correlation-id")

	res, err := s.client.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/setup/status: %v", err)
	}
	defer res.Body.Close()

	if got := res.Header.Get("X-Request-Id"); got != "systemtest-correlation-id" {
		t.Errorf("the response came back with %q, want the id that was sent", got)
	}

	if !s.waitForOutput(func(out string) bool {
		return strings.Contains(out, "systemtest-correlation-id")
	}) {
		t.Errorf("the id the caller sent is in nothing the server logged:\n%s", s.output())
	}
}

// The harness config runs with env prod, where the format defaults to json.
// Anything collecting these logs parses lines, so a stray plain-text one is a
// line that silently never gets indexed.
func TestProdWritesJSONLogs(t *testing.T) {
	s := startServer(t)

	if !s.waitForOutput(func(out string) bool {
		return strings.Contains(out, `"msg":"Starting server"`)
	}) {
		t.Errorf("the startup line is not json:\n%s", s.output())
	}
}
