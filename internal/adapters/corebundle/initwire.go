package corebundle

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/trust"
	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
)

const (
	scafldConfigHomeEnv = "SCAFLD_CONFIG_HOME"
	privateKeyRelPath   = "scafld/keys/receipt-ed25519"
	trustedKeysRelPath  = ".scafld/trusted-keys.json"
	initwireAssetRoot   = "assets/initwire"
)

// InitWire installs host accountability wiring and maintains the host signing key.
func InitWire(ctx context.Context, root string, installCI bool) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	var result Result
	publicKey, err := ensureHostKeypair()
	if err != nil {
		return Result{}, err
	}
	if err := upsertTrustedKey(root, publicKey, &result); err != nil {
		return Result{}, err
	}
	if err := installInitWireAssets(ctx, root, installCI, &result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func ensureHostKeypair() (ed25519.PublicKey, error) {
	path, err := HostPrivateKeyPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := os.Chmod(path, 0o600); err != nil {
			return nil, fmt.Errorf("chmod host signing key: %w", err)
		}
		privateKey, err := decodePrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("read host signing key: %w", err)
		}
		publicKey, ok := privateKey.Public().(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("derive host signing public key")
		}
		return publicKey, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read host signing key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create host signing key dir: %w", err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate host signing key: %w", err)
	}
	data = []byte(base64.StdEncoding.EncodeToString(privateKey))
	data = append(data, '\n')
	if err := atomicfile.Write(path, data, 0o600); err != nil {
		return nil, fmt.Errorf("write host signing key: %w", err)
	}
	return publicKey, nil
}

// HostPrivateKeyPath resolves the on-host receipt signing key path that scafld
// init generates. The gate signs with this same key, so init and the gate share
// one source of truth for the key location.
func HostPrivateKeyPath() (string, error) {
	configHome := strings.TrimSpace(os.Getenv(scafldConfigHomeEnv))
	if configHome == "" {
		var err error
		configHome, err = os.UserConfigDir()
		if err != nil {
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				return "", fmt.Errorf("resolve user config dir: %w", err)
			}
			configHome = filepath.Join(home, ".config")
		}
	}
	if !filepath.IsAbs(configHome) {
		abs, err := filepath.Abs(configHome)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", scafldConfigHomeEnv, err)
		}
		configHome = abs
	}
	return filepath.Join(configHome, filepath.FromSlash(privateKeyRelPath)), nil
}

func decodePrivateKey(data []byte) (ed25519.PrivateKey, error) {
	trimmed := strings.TrimSpace(string(data))
	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, err
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("ed25519 private key must be %d bytes, got %d", ed25519.PrivateKeySize, len(decoded))
	}
	return ed25519.PrivateKey(decoded), nil
}

func upsertTrustedKey(root string, publicKey ed25519.PublicKey, result *Result) error {
	keyID, err := trust.KeyIDFromRawEd25519PublicKey(publicKey)
	if err != nil {
		return err
	}
	encodedPublicKey := base64.StdEncoding.EncodeToString(publicKey)
	path := filepath.Join(root, filepath.FromSlash(trustedKeysRelPath))
	current, err := os.ReadFile(path)
	exists := true
	var keys trust.TrustedKeys
	if os.IsNotExist(err) {
		exists = false
		keys = trust.TrustedKeys{Version: trust.TrustedKeysVersion, Keys: []trust.TrustedKey{}}
	} else if err != nil {
		return fmt.Errorf("read %s: %w", trustedKeysRelPath, err)
	} else {
		keys, err = trust.ParseTrustedKeys(current)
		if err != nil {
			return fmt.Errorf("parse %s: %w", trustedKeysRelPath, err)
		}
	}
	for _, key := range keys.Keys {
		if key.KeyID == keyID {
			result.Skipped = append(result.Skipped, trustedKeysRelPath)
			return nil
		}
	}
	keys.Keys = append(keys.Keys, trust.TrustedKey{
		KeyID:     keyID,
		Alg:       trust.AlgorithmEd25519,
		PublicKey: encodedPublicKey,
	})
	next, err := trust.MarshalTrustedKeys(keys)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", trustedKeysRelPath, err)
	}
	if err := atomicfile.Write(path, next, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", trustedKeysRelPath, err)
	}
	if exists {
		result.Updated = append(result.Updated, trustedKeysRelPath)
	} else {
		result.Created = append(result.Created, trustedKeysRelPath)
	}
	return nil
}

func installInitWireAssets(ctx context.Context, root string, installCI bool, result *Result) error {
	entries, err := fs.ReadDir(assets, initwireAssetRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read embedded %s: %w", initwireAssetRoot, err)
	}
	if len(entries) == 0 {
		return nil
	}
	return fs.WalkDir(assets, initwireAssetRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(initwireAssetRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		data, err := assets.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		return installInitWireAsset(root, rel, data, installCI, result)
	})
}

func installInitWireAsset(root string, assetRel string, data []byte, installCI bool, result *Result) error {
	switch assetRel {
	case "mcp.json":
		return mergeJSONConfigFile(root, ".mcp.json", data, MergeMCPConfig, result)
	case "claude/settings.json":
		return mergeJSONConfigFile(root, ".claude/settings.json", data, MergeClaudeSettings, result)
	default:
		if strings.HasPrefix(assetRel, "ci/") {
			if !installCI {
				// CI merge gate is opt-in (scafld init --ci); local finalize needs no workflow.
				return nil
			}
			targetRel := ".github/workflows/" + strings.TrimPrefix(assetRel, "ci/")
			target := filepath.Join(root, filepath.FromSlash(targetRel))
			return writeManagedFile(target, targetRel, data, false, result)
		}
		targetRel := initWireTargetRel(assetRel)
		target := filepath.Join(root, filepath.FromSlash(targetRel))
		return writeManagedFile(target, targetRel, data, false, result)
	}
}

func initWireTargetRel(assetRel string) string {
	if strings.HasPrefix(assetRel, "claude/") {
		return ".claude/" + strings.TrimPrefix(assetRel, "claude/")
	}
	return assetRel
}

type mergeJSONFunc func(current []byte, desired []byte) ([]byte, bool, bool, error)

func mergeJSONConfigFile(root string, rel string, desired []byte, merge mergeJSONFunc, result *Result) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	current, err := os.ReadFile(path)
	exists := true
	if os.IsNotExist(err) {
		exists = false
		current = nil
	} else if err != nil {
		return fmt.Errorf("read %s: %w", rel, err)
	}
	next, changed, conflict, err := merge(current, desired)
	if err != nil {
		return err
	}
	if !changed {
		result.Skipped = append(result.Skipped, rel)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", rel, err)
	}
	if conflict && exists {
		if err := writeConflictBackup(path, current); err != nil {
			return fmt.Errorf("backup %s conflict: %w", rel, err)
		}
	}
	if err := atomicfile.Write(path, next, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	if exists {
		result.Updated = append(result.Updated, rel)
	} else {
		result.Created = append(result.Created, rel)
	}
	return nil
}

func writeConflictBackup(path string, current []byte) error {
	backupPath := path + ".scafld-conflict.bak"
	if existing, err := os.ReadFile(backupPath); err == nil && bytes.Equal(existing, current) {
		return nil
	}
	return atomicfile.Write(backupPath, current, 0o644)
}
