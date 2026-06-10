// Package sessionstore composes CLI session persistence with receipt trust
// checks from the committed workspace config.
package sessionstore

import (
	"context"

	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	"github.com/nilstate/scafld/v2/internal/adapters/jsonstore"
	"github.com/nilstate/scafld/v2/internal/adapters/trustcheck"
)

// New returns a JSON session store wired to replay receipt trust from the
// workspace's committed verify.trusted_keys_path when that config is valid.
func New(ctx context.Context, root string) jsonstore.SessionStore {
	trustedKeysPath := configadapter.Default().Verify.TrustedKeysPath
	if cfg, err := configadapter.LoadBase(ctx, root); err == nil {
		trustedKeysPath = cfg.Verify.TrustedKeysPath
	}
	return jsonstore.SessionStore{Root: root, TrustChecker: trustcheck.FromRoot(root, trustedKeysPath)}
}
