package trust

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	// TrustedKeysVersion is the current .scafld/trusted-keys.json schema version.
	TrustedKeysVersion = 1
	// AlgorithmEd25519 is the only signing algorithm supported by receipt keys.
	AlgorithmEd25519 = "ed25519"
)

// TrustedKeys is the .scafld/trusted-keys.json schema.
type TrustedKeys struct {
	Version int          `json:"version"`
	Keys    []TrustedKey `json:"keys"`
}

// TrustedKey is one raw Ed25519 public key allowlist entry.
type TrustedKey struct {
	KeyID     string `json:"key_id"`
	Alg       string `json:"alg"`
	PublicKey string `json:"public_key"`
	Revoked   bool   `json:"revoked,omitempty"`
}

// KeyIDFromRawEd25519PublicKey returns the stable receipt key id for a raw public key.
func KeyIDFromRawEd25519PublicKey(raw []byte) (string, error) {
	if len(raw) != ed25519.PublicKeySize {
		return "", fmt.Errorf("ed25519 public key must be %d bytes, got %d", ed25519.PublicKeySize, len(raw))
	}
	sum := sha256.Sum256(raw)
	return AlgorithmEd25519 + ":" + base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// ParseTrustedKeys parses and validates a trusted-key allowlist.
func ParseTrustedKeys(data []byte) (TrustedKeys, error) {
	var keys TrustedKeys
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&keys); err != nil {
		return TrustedKeys{}, fmt.Errorf("decode trusted keys: %w", err)
	}
	if err := keys.Validate(); err != nil {
		return TrustedKeys{}, err
	}
	return keys, nil
}

// MarshalTrustedKeys validates and marshals trusted keys with deterministic key order.
func MarshalTrustedKeys(keys TrustedKeys) ([]byte, error) {
	if err := keys.Validate(); err != nil {
		return nil, err
	}
	ordered := keys
	ordered.Keys = append([]TrustedKey(nil), keys.Keys...)
	sort.Slice(ordered.Keys, func(i, j int) bool {
		return ordered.Keys[i].KeyID < ordered.Keys[j].KeyID
	})
	data, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal trusted keys: %w", err)
	}
	return append(data, '\n'), nil
}

// Validate checks trusted-key schema and key identity integrity.
func (keys TrustedKeys) Validate() error {
	if keys.Version != TrustedKeysVersion {
		return fmt.Errorf("trusted keys version must be %d", TrustedKeysVersion)
	}
	seen := map[string]bool{}
	for idx, key := range keys.Keys {
		prefix := fmt.Sprintf("keys[%d]", idx)
		if strings.TrimSpace(key.KeyID) == "" {
			return fmt.Errorf("%s key_id is required", prefix)
		}
		if seen[key.KeyID] {
			return fmt.Errorf("duplicate trusted key id %q", key.KeyID)
		}
		seen[key.KeyID] = true
		if key.Alg != AlgorithmEd25519 {
			return fmt.Errorf("%s alg must be %q", prefix, AlgorithmEd25519)
		}
		raw, err := base64.StdEncoding.DecodeString(key.PublicKey)
		if err != nil {
			return fmt.Errorf("%s public_key must be base64 raw ed25519 public key: %w", prefix, err)
		}
		derived, err := KeyIDFromRawEd25519PublicKey(raw)
		if err != nil {
			return fmt.Errorf("%s public_key malformed: %w", prefix, err)
		}
		if derived != key.KeyID {
			return fmt.Errorf("%s key_id mismatch: got %q want %q", prefix, key.KeyID, derived)
		}
	}
	return nil
}

// ActiveKey returns the non-revoked trusted key matching id. It is the single
// authority for receipt key lookup: an unknown id or a revoked key is rejected
// here so callers never re-implement revocation. Revoked entries remain valid in
// the allowlist file (so revoking one key does not brick the others); they are
// rejected only at the moment a receipt tries to use them.
func (keys TrustedKeys) ActiveKey(id string) (TrustedKey, error) {
	for _, key := range keys.Keys {
		if key.KeyID != id {
			continue
		}
		if key.Revoked {
			return TrustedKey{}, fmt.Errorf("trusted key %q is revoked", id)
		}
		return key, nil
	}
	return TrustedKey{}, fmt.Errorf("unknown key_id %q", id)
}

// PublicKeyBytes decodes the raw Ed25519 public key.
func (key TrustedKey) PublicKeyBytes() ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(key.PublicKey)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("malformed ed25519 public key")
	}
	return raw, nil
}
