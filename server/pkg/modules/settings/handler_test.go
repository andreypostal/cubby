package settings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestTLSStatusEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	certPEM, keyPEM := selfSigned(t, newECDSAKey(t), time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	// Nothing configured, which is what the endpoint answers for most servers
	off := call(t, &Settings{}, httptest.NewRequest(http.MethodGet, "/settings/tls", nil))
	if off.Code != http.StatusOK {
		t.Fatalf("status without a certificate answered %d", off.Code)
	}

	if status(t, off.Body.Bytes()).Enabled {
		t.Fatal("TLS is reported as enabled without a certificate")
	}

	s := configured(t, certPEM, keyPEM)
	if err := s.load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	res := call(t, s, httptest.NewRequest(http.MethodGet, "/settings/tls", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status answered %d: %s", res.Code, res.Body)
	}

	reported := status(t, res.Body.Bytes())
	if !reported.Enabled || reported.Certificate == nil {
		t.Fatal("the configured certificate is not reported")
	}

	if reported.Certificate.Subject != "CN=lists.test" {
		t.Fatalf("unexpected subject %q", reported.Certificate.Subject)
	}

	if reported.Certificate.Expired {
		t.Fatal("a certificate valid for another hour is reported as expired")
	}

	if !reported.Certificate.SelfSigned || reported.Certificate.ChainLength != 1 {
		t.Fatalf("unexpected chain: %+v", reported.Certificate)
	}

	// The response describes the certificate, and must never carry the key
	if strings.Contains(res.Body.String(), "PRIVATE KEY") {
		t.Fatal("the response carries private key material")
	}
}

// The endpoint reports an expired certificate rather than hiding it, the server
// is still serving with it
func TestTLSStatusReportsAnExpiredCertificate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	certPEM, keyPEM := selfSigned(t, newECDSAKey(t), time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))

	s := configured(t, certPEM, keyPEM)
	if err := s.load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	res := call(t, s, httptest.NewRequest(http.MethodGet, "/settings/tls", nil))

	reported := status(t, res.Body.Bytes())
	if reported.Certificate == nil || !reported.Certificate.Expired {
		t.Fatalf("an expired certificate is not reported as such: %+v", reported.Certificate)
	}
}

// The module is read only, nothing may change the certificate over HTTP
func TestTLSHasNoWritingRoutes(t *testing.T) {
	routes, err := (&Settings{}).Routes()
	if err != nil {
		t.Fatalf("routes: %v", err)
	}

	for _, route := range routes {
		if route.Method != http.MethodGet {
			t.Fatalf("the module exposes %s %s", route.Method, route.Path)
		}
	}
}

func call(t *testing.T, s *Settings, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	engine := gin.New()
	routes, err := s.Routes()
	if err != nil {
		t.Fatalf("routes: %v", err)
	}

	for _, route := range routes {
		engine.Handle(route.Method, route.Path, route.Handlers...)
	}

	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)

	return res
}

func status(t *testing.T, body []byte) tlsStatusResponse {
	t.Helper()

	var res tlsStatusResponse
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("reading the response: %v", err)
	}

	return res
}
