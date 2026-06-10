package trust

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestKeyIDFromRawEd25519PublicKeyIsStable(t *testing.T) {
	t.Parallel()

	pub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("a", 64)))
	if err != nil {
		t.Fatal(err)
	}
	first, err := KeyIDFromRawEd25519PublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	second, err := KeyIDFromRawEd25519PublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second || !strings.HasPrefix(first, "ed25519:") {
		t.Fatalf("key id = %q %q", first, second)
	}
}

func TestParseTrustedKeysRejectsDuplicateMismatchedAndMalformedKeys(t *testing.T) {
	t.Parallel()

	pub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("b", 64)))
	if err != nil {
		t.Fatal(err)
	}
	id, err := KeyIDFromRawEd25519PublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	raw := base64.StdEncoding.EncodeToString(pub)
	valid := `{"version":1,"keys":[{"key_id":"` + id + `","alg":"ed25519","public_key":"` + raw + `"}]}`
	if _, err := ParseTrustedKeys([]byte(valid)); err != nil {
		t.Fatalf("valid trusted keys rejected: %v", err)
	}

	cases := map[string]string{
		"duplicate":  `{"version":1,"keys":[{"key_id":"` + id + `","alg":"ed25519","public_key":"` + raw + `"},{"key_id":"` + id + `","alg":"ed25519","public_key":"` + raw + `"}]}`,
		"mismatch":   `{"version":1,"keys":[{"key_id":"ed25519:wrong","alg":"ed25519","public_key":"` + raw + `"}]}`,
		"malformed":  `{"version":1,"keys":[{"key_id":"` + id + `","alg":"ed25519","public_key":"bad"}]}`,
		"wrong alg":  `{"version":1,"keys":[{"key_id":"` + id + `","alg":"rsa","public_key":"` + raw + `"}]}`,
		"bad schema": `{"version":2,"keys":[]}`,
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseTrustedKeys([]byte(text)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseTrustedKeysAllowsRevokedEntries(t *testing.T) {
	t.Parallel()

	pub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("c", 64)))
	if err != nil {
		t.Fatal(err)
	}
	id, err := KeyIDFromRawEd25519PublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	raw := base64.StdEncoding.EncodeToString(pub)
	text := `{"version":1,"keys":[{"key_id":"` + id + `","alg":"ed25519","public_key":"` + raw + `","revoked":true}]}`
	if _, err := ParseTrustedKeys([]byte(text)); err != nil {
		t.Fatalf("a revoked entry must still parse so it does not brick the allowlist: %v", err)
	}
}

func TestParseTrustedKeysLifecycleFields(t *testing.T) {
	t.Parallel()

	pub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("f", 64)))
	if err != nil {
		t.Fatal(err)
	}
	id, err := KeyIDFromRawEd25519PublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	raw := base64.StdEncoding.EncodeToString(pub)
	valid := `{"version":1,"keys":[{"key_id":"` + id + `","alg":"ed25519","public_key":"` + raw + `","revoked_at":"2026-06-04T00:00:00Z","expires_at":"2026-06-05T00:00:00Z"}]}`
	if _, err := ParseTrustedKeys([]byte(valid)); err != nil {
		t.Fatalf("valid lifecycle fields rejected: %v", err)
	}
	invalid := `{"version":1,"keys":[{"key_id":"` + id + `","alg":"ed25519","public_key":"` + raw + `","expires_at":"tomorrow"}]}`
	if _, err := ParseTrustedKeys([]byte(invalid)); err == nil || !strings.Contains(err.Error(), "expires_at") {
		t.Fatalf("bad lifecycle timestamp must be rejected, got %v", err)
	}
}

func TestActiveKeyRejectsRevokedAndUnknown(t *testing.T) {
	t.Parallel()

	activePub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("d", 64)))
	if err != nil {
		t.Fatal(err)
	}
	revokedPub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("e", 64)))
	if err != nil {
		t.Fatal(err)
	}
	activeID, err := KeyIDFromRawEd25519PublicKey(activePub)
	if err != nil {
		t.Fatal(err)
	}
	revokedID, err := KeyIDFromRawEd25519PublicKey(revokedPub)
	if err != nil {
		t.Fatal(err)
	}
	keys := TrustedKeys{Version: TrustedKeysVersion, Keys: []TrustedKey{
		{KeyID: activeID, Alg: AlgorithmEd25519, PublicKey: base64.StdEncoding.EncodeToString(activePub)},
		{KeyID: revokedID, Alg: AlgorithmEd25519, PublicKey: base64.StdEncoding.EncodeToString(revokedPub), Revoked: true},
	}}
	if err := keys.Validate(); err != nil {
		t.Fatalf("an allowlist mixing active and revoked keys must validate: %v", err)
	}
	if _, err := keys.ActiveKey(activeID); err != nil {
		t.Fatalf("active key must resolve even when another key is revoked: %v", err)
	}
	if _, err := keys.ActiveKey(revokedID); err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("revoked key must be rejected per-receipt, got %v", err)
	}
	if _, err := keys.ActiveKey("ed25519:nope"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown key must be rejected, got %v", err)
	}
}

func TestActiveKeyAtHonorsRevokedAtAndExpiresAt(t *testing.T) {
	t.Parallel()

	activePub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("g", 64)))
	if err != nil {
		t.Fatal(err)
	}
	revokedPub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("h", 64)))
	if err != nil {
		t.Fatal(err)
	}
	expiredPub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("i", 64)))
	if err != nil {
		t.Fatal(err)
	}
	activeID, err := KeyIDFromRawEd25519PublicKey(activePub)
	if err != nil {
		t.Fatal(err)
	}
	revokedID, err := KeyIDFromRawEd25519PublicKey(revokedPub)
	if err != nil {
		t.Fatal(err)
	}
	expiredID, err := KeyIDFromRawEd25519PublicKey(expiredPub)
	if err != nil {
		t.Fatal(err)
	}
	keys := TrustedKeys{Version: TrustedKeysVersion, Keys: []TrustedKey{
		{KeyID: activeID, Alg: AlgorithmEd25519, PublicKey: base64.StdEncoding.EncodeToString(activePub)},
		{KeyID: revokedID, Alg: AlgorithmEd25519, PublicKey: base64.StdEncoding.EncodeToString(revokedPub), RevokedAt: "2026-06-04T00:00:00Z"},
		{KeyID: expiredID, Alg: AlgorithmEd25519, PublicKey: base64.StdEncoding.EncodeToString(expiredPub), ExpiresAt: "2026-06-04T00:00:00Z"},
	}}
	if _, err := keys.ActiveKeyAt(activeID, "2026-06-04T00:00:00Z"); err != nil {
		t.Fatalf("active key rejected: %v", err)
	}
	if _, err := keys.ActiveKey(revokedID); err != nil {
		t.Fatalf("revoked_at alone must not change legacy ActiveKey semantics: %v", err)
	}
	if _, err := keys.ActiveKeyAt(revokedID, "2026-06-04T00:00:00Z"); err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("revoked_at key must be rejected at minted_at, got %v", err)
	}
	if _, err := keys.ActiveKeyAt(expiredID, "2026-06-04T00:00:00Z"); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expires_at key must be rejected at minted_at, got %v", err)
	}
	if _, err := keys.ActiveKeyAt(activeID, "not-time"); err == nil || !strings.Contains(err.Error(), "minted_at") {
		t.Fatalf("bad receipt minted_at must be rejected, got %v", err)
	}
}

func TestLifecycleSummaryCountsKeyStates(t *testing.T) {
	t.Parallel()

	keys := TrustedKeys{Version: TrustedKeysVersion, Keys: []TrustedKey{
		testKey(t, "j", TrustedKey{}),
		testKey(t, "k", TrustedKey{Revoked: true}),
		testKey(t, "l", TrustedKey{ExpiresAt: "2026-06-04T00:00:00Z"}),
	}}
	if err := keys.Validate(); err != nil {
		t.Fatal(err)
	}
	summary := keys.LifecycleSummary(time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC))
	if summary.Active != 1 || summary.Revoked != 1 || summary.Expired != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}

func testKey(t *testing.T, seed string, overrides TrustedKey) TrustedKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat(seed, 64)))
	if err != nil {
		t.Fatal(err)
	}
	id, err := KeyIDFromRawEd25519PublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	key := TrustedKey{KeyID: id, Alg: AlgorithmEd25519, PublicKey: base64.StdEncoding.EncodeToString(pub)}
	key.Revoked = overrides.Revoked
	key.RevokedAt = overrides.RevokedAt
	key.ExpiresAt = overrides.ExpiresAt
	return key
}
