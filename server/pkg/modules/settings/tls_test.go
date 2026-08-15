package settings

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadServesTheConfiguredPair(t *testing.T) {
	certPEM, keyPEM := selfSigned(t, newECDSAKey(t), time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	s := configured(t, certPEM, keyPEM)
	if err := s.load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	cert := s.Certificate()
	if cert == nil {
		t.Fatal("the configured certificate is not being served")
	}

	// Everything reading the certificate goes through the leaf, which the
	// standard library parses on the way in
	if cert.Leaf == nil {
		t.Fatal("the leaf certificate was not parsed")
	}

	if cert.Leaf.Subject.CommonName != "lists.test" {
		t.Fatalf("unexpected subject %q", cert.Leaf.Subject)
	}
}

// Nothing configured is the default and leaves the server on plain HTTP
func TestLoadWithoutConfiguredPathsStaysOnPlainHTTP(t *testing.T) {
	s := &Settings{}

	if err := s.load(); err != nil {
		t.Fatalf("load without any path: %v", err)
	}

	if s.Certificate() != nil {
		t.Fatal("a certificate appeared out of nowhere")
	}
}

// Everything the operator got wrong has to stop the server rather than let it
// come up on plain HTTP with whoever configured it thinking it is on TLS
func TestLoadRefuses(t *testing.T) {
	certPEM, keyPEM := selfSigned(t, newECDSAKey(t), time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	_, otherKeyPEM := selfSigned(t, newECDSAKey(t), time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	missing := filepath.Join(t.TempDir(), "nowhere.pem")

	cases := []struct {
		name     string
		settings *Settings
		message  string
	}{
		{
			name:     "only the certificate configured",
			settings: &Settings{certFile: writeFile(t, certPEM)},
			message:  "both files",
		},
		{
			name:     "only the private key configured",
			settings: &Settings{keyFile: writeFile(t, keyPEM)},
			message:  "both files",
		},
		{
			name:     "a certificate that is not there",
			settings: &Settings{certFile: missing, keyFile: writeFile(t, keyPEM)},
			message:  "nowhere.pem",
		},
		{
			name:     "a private key that is not there",
			settings: &Settings{certFile: writeFile(t, certPEM), keyFile: missing},
			message:  "nowhere.pem",
		},
		{
			name:     "a key that does not belong to the certificate",
			settings: configured(t, certPEM, otherKeyPEM),
			message:  "does not match",
		},
		{
			name:     "a file that is not a certificate at all",
			settings: configured(t, []byte("nothing to see here\n"), keyPEM),
			message:  "PEM",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.settings.load()
			if err == nil {
				t.Fatal("the configuration was accepted")
			}

			if !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error %q does not mention %q", err, tc.message)
			}

			if tc.settings.Certificate() != nil {
				t.Fatal("a certificate was served anyway")
			}
		})
	}
}

// An expired certificate is still the protocol the server was set up with, so
// it comes up on it. The status endpoint is where that is reported.
func TestLoadAcceptsAnExpiredCertificate(t *testing.T) {
	certPEM, keyPEM := selfSigned(t, newECDSAKey(t), time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))

	s := configured(t, certPEM, keyPEM)
	if err := s.load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	if s.Certificate() == nil {
		t.Fatal("an expired certificate took the server off TLS")
	}
}

// configured writes a pair where a server would find it and points a module at
// it, the way the configuration does
func configured(t *testing.T, certPEM, keyPEM []byte) *Settings {
	t.Helper()

	return &Settings{certFile: writeFile(t, certPEM), keyFile: writeFile(t, keyPEM)}
}

func writeFile(t *testing.T, data []byte) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "*.pem")
	if err != nil {
		t.Fatalf("writing a temporary file: %v", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		t.Fatalf("writing a temporary file: %v", err)
	}

	return f.Name()
}

func newECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}

	return key
}

// selfSigned builds a certificate for the given key and returns both in PEM
func selfSigned(t *testing.T, key *ecdsa.PrivateKey, notBefore, notAfter time.Time) ([]byte, []byte) {
	t.Helper()

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "lists.test"},
		DNSNames:     []string{"lists.test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatalf("creating the certificate: %v", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshalling the key: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}
