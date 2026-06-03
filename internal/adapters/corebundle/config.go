package corebundle

import (
	"context"
	"path/filepath"
)

func installProjectConfig(ctx context.Context, root string, result *Result) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := writeManagedFile(filepath.Join(root, ".scafld", "config.yaml"), ".scafld/config.yaml", projectConfig(), false, result); err != nil {
		return err
	}
	local := []byte(`# Local scafld overrides. This file should stay uncommitted.
#
# Keep this file to personal machine settings only. The full config shape lives
# in .scafld/core/config.yaml; the committed project config lives in
# .scafld/config.yaml.
#
# execution:
#   path_prepend:
#     - "$HOME/.rbenv/shims"
#     - "$HOME/.rbenv/bin"
# harden:
#   external:
#     provider: "codex"
#     codex:
#       model: "gpt-5.5"
#       # endpoint_host: "api.openai.com"
#     claude:
#       model: "claude-opus-4-7"
#       # endpoint_host: "api.anthropic.com"
#     gemini:
#       # model: ""
#       # endpoint_host: "generativelanguage.googleapis.com"
# review:
#   external:
#     provider: "codex"
#     codex:
#       model: "gpt-5.5"
#       # endpoint_host: "api.openai.com"
#     claude:
#       model: "claude-opus-4-7"
#       # endpoint_host: "api.anthropic.com"
#     gemini:
#       # model: ""
#       # endpoint_host: "generativelanguage.googleapis.com"
`)
	return writeManagedFile(filepath.Join(root, ".scafld", "config.local.yaml"), ".scafld/config.local.yaml", local, false, result)
}

func projectConfig() []byte {
	return []byte(`# scafld project configuration
#
# Keep this file sparse. Runtime defaults live in the scafld binary; the full
# example shape lives in .scafld/core/config.yaml.
#
# Put only project-specific invariant IDs, execution environment, review
# defaults, or review-pass overrides here. Use scafld config for
# evidence-backed suggestions before tightening this file.

version: "1.0"

invariants:
  canonical: {}

execution:
  path_prepend: []
  env: {}
`)
}
