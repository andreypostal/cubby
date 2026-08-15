package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"
)

// Nothing configured leaves the API where it has always been, on plain HTTP
func TestListenerServesPlainHTTPWithoutACertificate(t *testing.T) {
	addr := serve(t, nil)

	if body := get(t, plainClient(), "http://"+addr); body != "ok" {
		t.Fatalf("plain HTTP answered %q", body)
	}
}

// A configured certificate puts the same address on TLS, and only on TLS
func TestListenerServesTLSWithACertificate(t *testing.T) {
	cert, pool := serverCertificate(t)
	addr := serve(t, cert)

	if body := get(t, tlsClient(pool), "https://"+addr); body != "ok" {
		t.Fatalf("HTTPS answered %q", body)
	}

	// net/http recognizes a plaintext request on a TLS port and says so, which
	// is what a client left on the old address gets to see
	res, err := plainClient().Get("http://" + addr)
	if err != nil {
		t.Fatalf("plain GET against the TLS port: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("plain HTTP answered %s, expected 400", res.Status)
	}
}

// serve brings up a server the way [Api.Start] does and returns its address
func serve(t *testing.T, cert *tls.Certificate) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	addr := ln.Addr().String()

	if cert != nil {
		ln = tls.NewListener(ln, tlsConfig(cert))
	}

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			io.WriteString(w, "ok")
		}),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	t.Cleanup(func() { srv.Close() })

	go srv.Serve(ln)

	return addr
}

func get(t *testing.T, client *http.Client, url string) string {
	t.Helper()

	res, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("reading the body: %v", err)
	}

	return string(body)
}

// plainClient carries its own transport rather than sharing the default one
// with the other tests, whose keep-alive connections go to other servers
func plainClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{}}
}

// tlsClient trusts only the certificate the test generated, so a handshake that
// somehow served something else fails the test instead of passing quietly
func tlsClient(pool *x509.CertPool) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "lists.test"},
		},
	}
}

func serverCertificate(t *testing.T) (*tls.Certificate, *x509.CertPool) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "lists.test"},
		DNSNames:     []string{"lists.test"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatalf("creating the certificate: %v", err)
	}

	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parsing the certificate: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(leaf)

	return &tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
		Leaf:        leaf,
	}, pool
}
