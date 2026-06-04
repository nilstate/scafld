package initcmd

import (
	"context"
	"fmt"

	"github.com/nilstate/scafld/v2/internal/adapters/corebundle"
	"github.com/nilstate/scafld/v2/internal/adapters/filesystem"
	"github.com/nilstate/scafld/v2/internal/app/bootstrap"
	"github.com/nilstate/scafld/v2/internal/core/workspace"
)

// Run initializes a workspace and installs bundled project assets. The CI
// merge-gate workflow is installed only when installCI is set (scafld init --ci);
// by default init is local-only and writes no .github/workflows asset.
func Run(ctx context.Context, root string, agentDocs bool, installCI bool) (workspace.InitResult, error) {
	result, err := bootstrap.Run(ctx, filesystem.WorkspaceStore{}, bootstrap.Input{Root: root})
	if err != nil {
		return workspace.InitResult{}, fmt.Errorf("init workspace: %w", err)
	}
	bundle, err := corebundle.Init(ctx, result.Root)
	if err != nil {
		return workspace.InitResult{}, fmt.Errorf("install core bundle: %w", err)
	}
	result.Merge(bundle.Created, bundle.Updated, bundle.Skipped)
	initWire, err := corebundle.InitWire(ctx, result.Root, installCI)
	if err != nil {
		return workspace.InitResult{}, fmt.Errorf("install init wiring: %w", err)
	}
	result.Merge(initWire.Created, initWire.Updated, initWire.Skipped)
	if agentDocs {
		agentDocsResult, err := corebundle.InitAgentDocs(ctx, result.Root)
		if err != nil {
			return workspace.InitResult{}, fmt.Errorf("install agent docs: %w", err)
		}
		result.Merge(agentDocsResult.Created, agentDocsResult.Updated, agentDocsResult.Skipped)
	}
	gitignore, err := corebundle.InitGitignore(ctx, result.Root)
	if err != nil {
		return workspace.InitResult{}, fmt.Errorf("install gitignore: %w", err)
	}
	result.Merge(gitignore.Created, gitignore.Updated, gitignore.Skipped)
	return result, nil
}

// Message renders the human-facing init summary, naming whether the opt-in CI
// merge gate was installed so a default (local-only) init advertises the upgrade.
func Message(result workspace.InitResult, installCI bool) string {
	message := bootstrap.Message(result)
	if installCI {
		return message + "\nCI merge gate: installed .github/workflows/scafld-verify.yml. Run scafld verify --self-check to inspect wiring."
	}
	return message + "\nCI merge gate: not installed (local finalize only). Run scafld init --ci to add the PR verify workflow."
}
