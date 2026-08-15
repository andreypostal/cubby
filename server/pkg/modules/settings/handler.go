package settings

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// tlsStatusResponse holds information about the certificate in use
type tlsStatusResponse struct {
	Enabled     bool             `json:"enabled"`
	Certificate *certificateInfo `json:"certificate,omitempty"`
}

type certificateInfo struct {
	Subject     string    `json:"subject"`
	Issuer      string    `json:"issuer"`
	DNSNames    []string  `json:"dns_names"`
	IPAddresses []string  `json:"ip_addresses"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	// Fingerprint is the SHA-256 of the leaf certificate
	Fingerprint string    `json:"fingerprint"`
	ChainLength int       `json:"chain_length"`
	SelfSigned  bool      `json:"self_signed"`
	Expired     bool      `json:"expired"`
	LoadedAt    time.Time `json:"loaded_at"`
}

func (s *Settings) handleTLSStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.status())
}

func (s *Settings) status() tlsStatusResponse {
	if s.material == nil {
		return tlsStatusResponse{Enabled: false}
	}

	leaf := s.material.cert.Leaf

	ips := make([]string, 0, len(leaf.IPAddresses))
	for _, ip := range leaf.IPAddresses {
		ips = append(ips, ip.String())
	}

	names := leaf.DNSNames
	if names == nil {
		names = []string{}
	}

	return tlsStatusResponse{
		Enabled: true,
		Certificate: &certificateInfo{
			Subject:     leaf.Subject.String(),
			Issuer:      leaf.Issuer.String(),
			DNSNames:    names,
			IPAddresses: ips,
			NotBefore:   leaf.NotBefore,
			NotAfter:    leaf.NotAfter,
			Fingerprint: fingerprint(leaf.Raw),
			ChainLength: len(s.material.cert.Certificate),
			SelfSigned:  bytes.Equal(leaf.RawIssuer, leaf.RawSubject),
			Expired:     time.Now().After(leaf.NotAfter),
			LoadedAt:    s.material.loadedAt,
		},
	}
}

func fingerprint(der []byte) string {
	sum := sha256.Sum256(der)

	var b bytes.Buffer
	for i, octet := range sum {
		if i > 0 {
			b.WriteByte(':')
		}
		fmt.Fprintf(&b, "%02X", octet)
	}

	return b.String()
}
