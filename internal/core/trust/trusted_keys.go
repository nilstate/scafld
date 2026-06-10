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
	"time"
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
	RevokedAt string `json:"revoked_at,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// KeyLifecycleSummary counts trusted-key states at one point in time.
type KeyLifecycleSummary struct {
	Active  int
	Revoked int
	Expired int
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
		if _, err := parseOptionalTime(prefix, "revoked_at", key.RevokedAt); err != nil {
			return err
		}
		if _, err := parseOptionalTime(prefix, "expires_at", key.ExpiresAt); err != nil {
			return err
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

// ActiveKeyAt returns the trusted key matching id when it was valid at mintedAt.
// mintedAt must be RFC3339 because receipt verification evaluates key lifecycle
// against the signed receipt time, not against wall-clock now.
func (keys TrustedKeys) ActiveKeyAt(id string, mintedAt string) (TrustedKey, error) {
	key, err := keys.ActiveKey(id)
	if err != nil {
		return TrustedKey{}, err
	}
	minted, err := time.Parse(time.RFC3339, strings.TrimSpace(mintedAt))
	if err != nil {
		return TrustedKey{}, fmt.Errorf("receipt minted_at must be RFC3339 for key lifecycle check: %w", err)
	}
	state, reason, err := key.LifecycleStateAt(minted)
	if err != nil {
		return TrustedKey{}, err
	}
	if state != "active" {
		return TrustedKey{}, fmt.Errorf("trusted key %q is %s at minted_at: %s", id, state, reason)
	}
	return key, nil
}

// LifecycleSummary returns active/revoked/expired key counts at one point in time.
func (keys TrustedKeys) LifecycleSummary(at time.Time) KeyLifecycleSummary {
	var out KeyLifecycleSummary
	for _, key := range keys.Keys {
		state, _, err := key.LifecycleStateAt(at)
		if err != nil {
			continue
		}
		switch state {
		case "revoked":
			out.Revoked++
		case "expired":
			out.Expired++
		default:
			out.Active++
		}
	}
	return out
}

// LifecycleStateAt reports whether key is active, revoked, or expired at at.
func (key TrustedKey) LifecycleStateAt(at time.Time) (state string, reason string, err error) {
	if key.Revoked {
		return "revoked", "revoked flag is true", nil
	}
	if revokedAt, err := parseOptionalTime("key", "revoked_at", key.RevokedAt); err != nil {
		return "", "", err
	} else if !revokedAt.IsZero() && !at.Before(revokedAt) {
		return "revoked", "revoked_at " + revokedAt.UTC().Format(time.RFC3339), nil
	}
	if expiresAt, err := parseOptionalTime("key", "expires_at", key.ExpiresAt); err != nil {
		return "", "", err
	} else if !expiresAt.IsZero() && !at.Before(expiresAt) {
		return "expired", "expires_at " + expiresAt.UTC().Format(time.RFC3339), nil
	}
	return "active", "", nil
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

func parseOptionalTime(prefix string, field string, value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s %s must be RFC3339: %w", prefix, field, err)
	}
	return parsed, nil
}
