package corebundle

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/trust"
)

func TestInitWireCreatesHostKeyAndTrustedKeyIdempotently(t *testing.T) {
	root := t.TempDir()
	configHome := t.TempDir()
	t.Setenv(scafldConfigHomeEnv, configHome)

	result, err := InitWire(t.Context(), root, false)
	if err != nil {
		t.Fatal(err)
	}
	if !containsPath(result.Created, trustedKeysRelPath) {
		t.Fatalf("created = %v, want trusted keys", result.Created)
	}
	keyPath := filepath.Join(configHome, filepath.FromSlash(privateKeyRelPath))
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %o, want 0600", info.Mode().Perm())
	}
	if rel, err := filepath.Rel(root, keyPath); err != nil || rel == "." || !strings.HasPrefix(rel, "..") {
		t.Fatalf("private key path %q should be outside root %q (rel=%q err=%v)", keyPath, root, rel, err)
	}
	firstPrivateKeyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	keys := readTrustedKeys(t, root)
	if len(keys.Keys) != 1 {
		t.Fatalf("trusted keys = %+v, want one key", keys.Keys)
	}
	privateKey := decodeTestPrivateKey(t, firstPrivateKeyData)
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		t.Fatal("failed to derive public key")
	}
	if keys.Keys[0].PublicKey != base64.StdEncoding.EncodeToString(publicKey) {
		t.Fatalf("trusted public key does not match private key")
	}

	second, err := InitWire(t.Context(), root, false)
	if err != nil {
		t.Fatal(err)
	}
	secondPrivateKeyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstPrivateKeyData, secondPrivateKeyData) {
		t.Fatalf("private key changed on second init")
	}
	keys = readTrustedKeys(t, root)
	if len(keys.Keys) != 1 {
		t.Fatalf("trusted keys duplicated after second init: %+v", keys.Keys)
	}
	if !containsPath(second.Skipped, trustedKeysRelPath) {
		t.Fatalf("second skipped = %v, want trusted keys skipped", second.Skipped)
	}
}

func TestMergeMCPConfigPreservesUnrelatedEntriesAndReportsConflict(t *testing.T) {
	current := []byte(`{
  "custom": true,
  "mcpServers": {
    "other": {"command": "other"},
    "scafld": {"command": "old", "args": ["wrong"]}
  }
}`)
	desired := mustReadAsset(t, "assets/initwire/mcp.json")
	merged, changed, conflict, err := MergeMCPConfig(current, desired)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || !conflict {
		t.Fatalf("changed=%v conflict=%v, want true true", changed, conflict)
	}
	object := decodeJSONObject(t, merged)
	if object["custom"] != true {
		t.Fatalf("custom entry lost: %s", merged)
	}
	servers := object["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Fatalf("other server lost: %s", merged)
	}
	if !strings.Contains(string(merged), `"scafld"`) || !strings.Contains(string(merged), `"finalize"`) {
		t.Fatalf("scafld finalize server/tool missing: %s", merged)
	}

	again, changed, conflict, err := MergeMCPConfig(merged, desired)
	if err != nil {
		t.Fatal(err)
	}
	if changed || conflict || !bytes.Equal(merged, again) {
		t.Fatalf("merge not idempotent changed=%v conflict=%v", changed, conflict)
	}
}

func TestMergeClaudeSettingsPreservesUnrelatedEntriesAndReportsConflict(t *testing.T) {
	current := []byte(`{
  "permissions": {"allow": ["Read"]},
  "hooks": {
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo pre"}]}],
    "Stop": [
      {"name": "custom_stop", "hooks": [{"type": "command", "command": "echo custom"}]},
      {"name": "finalize-stop", "hooks": [{"type": "command", "command": "old"}]}
    ]
  }
}`)
	desired := mustReadAsset(t, "assets/initwire/claude/settings.json")
	merged, changed, conflict, err := MergeClaudeSettings(current, desired)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || !conflict {
		t.Fatalf("changed=%v conflict=%v, want true true", changed, conflict)
	}
	text := string(merged)
	for _, want := range []string{"permissions", "PreToolUse", "custom_stop", "finalize-stop", "scafld finalize --json --stdin"} {
		if !strings.Contains(text, want) {
			t.Fatalf("merged settings missing %q:\n%s", want, merged)
		}
	}
	if strings.Count(text, "finalize-stop") != 1 {
		t.Fatalf("scafld Stop hook duplicated:\n%s", merged)
	}
}

func TestInitWireDefaultMCPIsGateOnly(t *testing.T) {
	root := t.TempDir()
	configHome := t.TempDir()
	t.Setenv(scafldConfigHomeEnv, configHome)

	if _, err := InitWire(t.Context(), root, false); err != nil {
		t.Fatal(err)
	}
	mcp := mustReadFile(t, filepath.Join(root, ".mcp.json"))
	object := decodeJSONObject(t, mcp)
	servers := object["mcpServers"].(map[string]any)
	if len(servers) != 1 {
		t.Fatalf("mcp servers = %v, want finalize-only default", servers)
	}
	if _, ok := servers["scafld"]; !ok {
		t.Fatalf("scafld server missing from default .mcp.json:\n%s", mcp)
	}
	scafldServer := servers["scafld"].(map[string]any)
	includeTools := scafldServer["includeTools"].([]any)
	if len(includeTools) != 1 || includeTools[0] != "finalize" {
		t.Fatalf("default .mcp.json includeTools = %v, want finalize only", includeTools)
	}
	for _, denied := range []string{"harden-submit-stdio", "review-submit-stdio"} {
		if strings.Contains(string(mcp), denied) {
			t.Fatalf("default .mcp.json includes lifecycle tool %q:\n%s", denied, mcp)
		}
	}
}

func TestInitWireDefaultSkipsCIWorkflowAndInstallsLocalAssets(t *testing.T) {
	root := t.TempDir()
	configHome := t.TempDir()
	t.Setenv(scafldConfigHomeEnv, configHome)

	// Default init (no --ci) is local-only: it must not write the CI merge-gate workflow.
	if _, err := InitWire(t.Context(), root, false); err != nil {
		t.Fatal(err)
	}
	workflowPath := filepath.Join(root, ".github", "workflows", "scafld-verify.yml")
	if _, err := os.Stat(workflowPath); !os.IsNotExist(err) {
		t.Fatalf("default init wrote CI workflow %q (stat err=%v), want it skipped", workflowPath, err)
	}
	// Non-CI assets still install by default.
	for _, want := range []string{".claude/skills/finalize/SKILL.md", ".claude/commands/finalize.md", ".mcp.json"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(want))); err != nil {
			t.Fatalf("default init missing local asset %s: %v", want, err)
		}
	}

	// Opting in later with --ci adds the workflow idempotently into the same workspace.
	if _, err := InitWire(t.Context(), root, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(workflowPath); err != nil {
		t.Fatalf("scafld init --ci did not add the verify workflow on re-run: %v", err)
	}
}

func TestInitWireExistingJSONPreservesEntriesAndInstallsAssets(t *testing.T) {
	root := t.TempDir()
	configHome := t.TempDir()
	t.Setenv(scafldConfigHomeEnv, configHome)
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{"other":{"command":"other"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(`{"theme":"dark","hooks":{"Stop":[{"name":"custom_stop","hooks":[{"type":"command","command":"echo custom"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := InitWire(t.Context(), root, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{".mcp.json", ".claude/settings.json"} {
		if !containsPath(result.Updated, want) {
			t.Fatalf("updated = %v, want %s", result.Updated, want)
		}
	}
	for _, want := range []string{".claude/skills/finalize/SKILL.md", ".claude/commands/finalize.md"} {
		if !containsPath(result.Created, want) {
			t.Fatalf("created = %v, want %s", result.Created, want)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(want))); err != nil {
			t.Fatal(err)
		}
	}
	mcp := mustReadFile(t, filepath.Join(root, ".mcp.json"))
	if !strings.Contains(string(mcp), "other") || !strings.Contains(string(mcp), `"scafld"`) || !strings.Contains(string(mcp), `"finalize"`) {
		t.Fatalf(".mcp.json did not preserve and upsert:\n%s", mcp)
	}
	settings := mustReadFile(t, filepath.Join(root, ".claude", "settings.json"))
	for _, want := range []string{"theme", "custom_stop", "finalize-stop"} {
		if !strings.Contains(string(settings), want) {
			t.Fatalf("settings missing %q:\n%s", want, settings)
		}
	}
	workflow := mustReadFile(t, filepath.Join(root, ".github", "workflows", "scafld-verify.yml"))
	for _, want := range []string{"pull_request_target", "actions/setup-go", "SCAFLD_VERSION", "SCAFLD_VERIFY_TARGET", "RUNNER_TEMP", "git show", "scripts/scafld-verify.sh", "pull_request.head.sha"} {
		if !strings.Contains(string(workflow), want) {
			t.Fatalf("verify workflow missing %q:\n%s", want, workflow)
		}
	}
	scriptPath := filepath.Join(root, "scripts", "scafld-verify.sh")
	script := mustReadFile(t, scriptPath)
	for _, want := range []string{"SCAFLD_TRUSTED_KEYS", "git show \"$target:.scafld/trusted-keys.json\"", "go install", "--trusted-keys", ".scafld/receipts/latest.json"} {
		if !strings.Contains(string(script), want) {
			t.Fatalf("verify script missing %q:\n%s", want, script)
		}
	}
	if info, err := os.Stat(scriptPath); err != nil {
		t.Fatal(err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("verify script mode = %o, want executable", info.Mode().Perm())
	}
}

func readTrustedKeys(t *testing.T, root string) trust.TrustedKeys {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(trustedKeysRelPath)))
	if err != nil {
		t.Fatal(err)
	}
	keys, err := trust.ParseTrustedKeys(data)
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func decodeTestPrivateKey(t *testing.T, data []byte) ed25519.PrivateKey {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != ed25519.PrivateKeySize {
		t.Fatalf("private key length = %d, want %d", len(raw), ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(raw)
}

func mustReadAsset(t *testing.T, path string) []byte {
	t.Helper()
	data, err := assets.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func decodeJSONObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	return object
}
