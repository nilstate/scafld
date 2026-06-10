// Package trustcheck adapts trusted-key files to the session replay trust port.
package trustcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/trust"
)

// Checker validates receipt signatures against trusted-key lifecycle metadata.
type Checker struct {
	Keys    trust.TrustedKeys
	LoadErr error
}

// FromFile returns a checker for path. Read or parse failures are retained and
// reported only if replay actually encounters a receipt entry.
func FromFile(path string) Checker {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Checker{LoadErr: fmt.Errorf("read trusted keys: %w", err)}
	}
	keys, err := trust.ParseTrustedKeys(data)
	if err != nil {
		return Checker{LoadErr: err}
	}
	return Checker{Keys: keys}
}

// FromRoot resolves configuredPath below root unless it is absolute.
func FromRoot(root string, configuredPath string) Checker {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	path := strings.TrimSpace(configuredPath)
	if path == "" {
		path = ".scafld/trusted-keys.json"
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	return FromFile(path)
}

// CheckReceiptTrust implements session.ReceiptTrustChecker.
func (c Checker) CheckReceiptTrust(envelope receipt.Envelope) error {
	if c.LoadErr != nil {
		return c.LoadErr
	}
	_, err := c.Keys.ActiveKeyAt(envelope.Signature.KeyID, envelope.Body.MintedAt)
	return err
}
