package sign

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/trust"
)

func TestEd25519SignerSignsAndVerifies(t *testing.T) {
	t.Parallel()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "signing.key")
	if err := os.WriteFile(keyPath, priv, 0o600); err != nil {
		t.Fatal(err)
	}
	signer := Ed25519Signer{PrivateKeyPath: keyPath}

	canonical := []byte(`{"task_id":"t","verdict":"pass"}`)
	sig, err := signer.SignCanonical(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if sig.Alg != receipt.SignatureAlgorithm {
		t.Fatalf("alg = %q want %q", sig.Alg, receipt.SignatureAlgorithm)
	}
	wantID, err := trust.KeyIDFromRawEd25519PublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	if sig.KeyID != wantID {
		t.Fatalf("key_id = %q want %q", sig.KeyID, wantID)
	}
	raw, err := base64.StdEncoding.DecodeString(sig.Sig)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(pub, canonical, raw) {
		t.Fatal("signature does not verify against the derived public key")
	}
	// A tampered body must not verify under the same signature.
	if ed25519.Verify(pub, []byte(`{"task_id":"t","verdict":"fail"}`), raw) {
		t.Fatal("signature verified over tampered canonical bytes")
	}
}

func TestEd25519SignerRejectsMissingKey(t *testing.T) {
	t.Parallel()

	signer := Ed25519Signer{PrivateKeyPath: filepath.Join(t.TempDir(), "absent.key")}
	if _, err := signer.SignCanonical([]byte("x")); err == nil {
		t.Fatal("signing with a missing private key must fail closed")
	}
}
