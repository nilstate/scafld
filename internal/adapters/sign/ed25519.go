package sign

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/trust"
)

// Ed25519Signer loads an on-host private key and signs canonical receipt bytes.
type Ed25519Signer struct {
	PrivateKeyPath string
}

// Sign signs the canonical receipt body and returns its detached signature.
func (s Ed25519Signer) Sign(body receipt.Body) (receipt.DetachedSignature, error) {
	canonical, err := receipt.CanonicalBody(body)
	if err != nil {
		return receipt.DetachedSignature{}, err
	}
	return s.SignCanonical(canonical)
}

// SignCanonical signs caller-supplied canonical receipt bytes.
func (s Ed25519Signer) SignCanonical(canonical []byte) (receipt.DetachedSignature, error) {
	privateKey, err := loadEd25519PrivateKey(s.PrivateKeyPath)
	if err != nil {
		return receipt.DetachedSignature{}, err
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return receipt.DetachedSignature{}, fmt.Errorf("derive ed25519 public key")
	}
	keyID, err := trust.KeyIDFromRawEd25519PublicKey(publicKey)
	if err != nil {
		return receipt.DetachedSignature{}, err
	}
	signature := ed25519.Sign(privateKey, canonical)
	return receipt.DetachedSignature{
		Alg:   receipt.SignatureAlgorithm,
		KeyID: keyID,
		Sig:   base64.StdEncoding.EncodeToString(signature),
	}, nil
}

func loadEd25519PrivateKey(path string) (ed25519.PrivateKey, error) {
	if err := checkPrivateKeyMode(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ed25519 private key: %w", err)
	}
	trimmed := []byte(strings.TrimSpace(string(data)))
	if block, _ := pem.Decode(trimmed); block != nil {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse pkcs8 ed25519 private key: %w", err)
		}
		privateKey, ok := key.(ed25519.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not ed25519")
		}
		return privateKey, nil
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.RawURLEncoding} {
		if decoded, err := encoding.DecodeString(string(trimmed)); err == nil {
			return normalizePrivateKey(decoded)
		}
	}
	return normalizePrivateKey(data)
}

// checkPrivateKeyMode refuses group- or world-accessible signing keys so a
// shared host cannot read the key and forge receipts. Windows file modes do
// not map onto POSIX permission bits, so the check is unix-only.
func checkPrivateKeyMode(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat ed25519 private key: %w", err)
	}
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		return fmt.Errorf("ed25519 private key %s has mode %04o; require 0600 (chmod 600 to fix)", path, mode)
	}
	return nil
}

func normalizePrivateKey(data []byte) (ed25519.PrivateKey, error) {
	switch len(data) {
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(append([]byte(nil), data...)), nil
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(data), nil
	default:
		return nil, fmt.Errorf("ed25519 private key must be %d raw bytes or %d seed bytes, got %d", ed25519.PrivateKeySize, ed25519.SeedSize, len(data))
	}
}
