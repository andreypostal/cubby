package settings

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// load reads the pair named in the configuration
func (s *Settings) load() error {
	if s.certFile == "" && s.keyFile == "" {
		return nil
	}

	if s.certFile == "" || s.keyFile == "" {
		return errors.New(
			"TLS needs both files, set api.tls.cert_file and api.tls.key_file or neither",
		)
	}

	// Reads both files, builds the chain and turns away a key that
	// does not belong to the certificate it came with
	cert, err := tls.LoadX509KeyPair(s.certFile, s.keyFile)
	if err != nil {
		return fmt.Errorf("the configured TLS certificate cannot be used: %w", err)
	}

	loadedAt := time.Now()
	if info, err := os.Stat(s.certFile); err == nil {
		loadedAt = info.ModTime()
	}

	s.material = &tlsMaterial{cert: &cert, loadedAt: loadedAt}

	if time.Now().After(cert.Leaf.NotAfter) {
		slog.Warn("The configured TLS certificate has expired",
			slog.String("subject", cert.Leaf.Subject.String()),
			slog.Time("expired_at", cert.Leaf.NotAfter),
		)
	}

	slog.Info("Loaded the configured TLS certificate",
		slog.String("subject", cert.Leaf.Subject.String()),
		slog.Time("expires_at", cert.Leaf.NotAfter),
	)

	return nil
}
