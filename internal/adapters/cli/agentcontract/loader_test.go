package agentcontract

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corecontract "github.com/nilstate/scafld/v2/internal/core/agentcontract"
)

func TestLoadAllEmbeddedRoleContracts(t *testing.T) {
	t.Parallel()

	for _, role := range corecontract.Roles() {
		contract, err := Load(context.Background(), t.TempDir(), role)
		if err != nil {
			t.Fatalf("load %s: %v", role, err)
		}
		if contract.Role != role || contract.Body == "" || contract.SHA256 == "" || contract.Bytes == 0 {
			t.Fatalf("contract %s = %+v", role, contract)
		}
		if !strings.HasPrefix(contract.Path, "embedded:.scafld/core/prompts/") {
			t.Fatalf("embedded contract path = %q", contract.Path)
		}
	}
}

func TestLoadPrefersProjectPromptOverride(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld", "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "prompts", "review.md"), []byte("# Project Review\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	contract, err := Load(context.Background(), root, corecontract.RoleReview)
	if err != nil {
		t.Fatal(err)
	}
	if contract.Path != ".scafld/prompts/review.md" || !strings.Contains(contract.Body, "Project Review") {
		t.Fatalf("contract = %+v", contract)
	}
}
