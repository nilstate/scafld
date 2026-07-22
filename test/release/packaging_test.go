package release

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestModulePathIsPrimaryRepository(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.Contains(content, "module github.com/nilstate/scafld/v2\n") {
		t.Fatalf("go.mod must use primary module path:\n%s", data)
	}
}

func TestNpmPackageIsThinCliWrapper(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "package", "npm", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Name       string            `json:"name"`
		Bin        map[string]string `json:"bin"`
		Repository struct {
			URL string `json:"url"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg.Name != "scafld" || pkg.Bin["scafld"] != "bin/scafld.js" {
		t.Fatalf("unexpected npm package shape: %+v", pkg)
	}
	if !strings.Contains(pkg.Repository.URL, "github.com/nilstate/scafld") {
		t.Fatalf("repository must point at primary repo: %s", pkg.Repository.URL)
	}
}

func TestPyPIPackageIsThinCliWrapper(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "package", "pypi", "pyproject.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`name = "scafld"`,
		`scafld = "scafld_launcher.cli:main"`,
		`Repository = "https://github.com/nilstate/scafld"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("pyproject missing %q:\n%s", want, text)
		}
	}
}

func TestRegistryPackageVersionsStayInSync(t *testing.T) {
	t.Parallel()

	npmData, err := os.ReadFile(filepath.Join("..", "..", "package", "npm", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var npmPkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(npmData, &npmPkg); err != nil {
		t.Fatal(err)
	}
	pyproject, err := os.ReadFile(filepath.Join("..", "..", "package", "pypi", "pyproject.toml"))
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := os.ReadFile(filepath.Join("..", "..", "package", "pypi", "src", "scafld_launcher", "__init__.py"))
	if err != nil {
		t.Fatal(err)
	}
	verifyScript, err := os.ReadFile(filepath.Join("..", "..", "scripts", "scafld-verify.sh"))
	if err != nil {
		t.Fatal(err)
	}
	bundledVerifyScript, err := os.ReadFile(filepath.Join("..", "..", "internal", "adapters", "corebundle", "assets", "initwire", "scripts", "scafld-verify.sh"))
	if err != nil {
		t.Fatal(err)
	}
	verifyAction, err := os.ReadFile(filepath.Join("..", "..", ".github", "actions", "scafld-verify", "action.yml"))
	if err != nil {
		t.Fatal(err)
	}
	verifyWorkflow, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "scafld-verify.yml"))
	if err != nil {
		t.Fatal(err)
	}
	bundledVerifyWorkflow, err := os.ReadFile(filepath.Join("..", "..", "internal", "adapters", "corebundle", "assets", "initwire", "ci", "scafld-verify.yml"))
	if err != nil {
		t.Fatal(err)
	}
	pyVersion := mustMatchVersion(t, string(pyproject), `(?m)^version = "([^"]+)"`)
	launcherVersion := mustMatchVersion(t, string(launcher), `(?m)^\s*__version__ = "([^"]+)"`)
	verifyVersion := strings.TrimPrefix(mustMatchVersion(t, string(verifyScript), `(?m)^version="\$\{SCAFLD_VERSION:-v([^}]+)\}"`), "v")
	bundledVerifyVersion := strings.TrimPrefix(mustMatchVersion(t, string(bundledVerifyScript), `(?m)^version="\$\{SCAFLD_VERSION:-v([^}]+)\}"`), "v")
	actionVersion := strings.TrimPrefix(mustMatchVersion(t, string(verifyAction), `(?m)^\s+default: v([^\s]+)`), "v")
	workflowVersion := strings.TrimPrefix(mustMatchVersion(t, string(verifyWorkflow), `(?m)^\s+SCAFLD_VERSION: v([^\s]+)`), "v")
	bundledWorkflowVersion := strings.TrimPrefix(mustMatchVersion(t, string(bundledVerifyWorkflow), `(?m)^\s+SCAFLD_VERSION: v([^\s]+)`), "v")
	if npmPkg.Version == "" {
		t.Fatal("npm package version is empty")
	}
	if npmPkg.Version != pyVersion || npmPkg.Version != launcherVersion || npmPkg.Version != verifyVersion || npmPkg.Version != bundledVerifyVersion || npmPkg.Version != actionVersion || npmPkg.Version != workflowVersion || npmPkg.Version != bundledWorkflowVersion {
		t.Fatalf("package versions drifted: npm=%s pyproject=%s launcher=%s verify=%s bundled_verify=%s action=%s workflow=%s bundled_workflow=%s", npmPkg.Version, pyVersion, launcherVersion, verifyVersion, bundledVerifyVersion, actionVersion, workflowVersion, bundledWorkflowVersion)
	}
}

func TestVerifierWorkflowsUseTrustedBaseSplitPullRequestTargetLanes(t *testing.T) {
	t.Parallel()

	for _, file := range []string{
		filepath.Join("..", "..", ".github", "workflows", "scafld-verify.yml"),
		filepath.Join("..", "..", "internal", "adapters", "corebundle", "assets", "initwire", "ci", "scafld-verify.yml"),
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "pull_request_target:") {
			t.Fatalf("%s must use pull_request_target so PR-authored workflow changes cannot alter verifier logic", file)
		}
		if strings.Contains(text, "\n  pull_request:\n") {
			t.Fatalf("%s must not run verifier logic from PR-authored pull_request workflow files", file)
		}
		for _, forbidden := range []string{
			"repository: ${{ github.event.pull_request.head.repo.full_name",
			"ref: ${{ github.event.pull_request.head.sha",
			"allow-unsafe-pr-checkout: true",
			"secrets.",
			"github.token",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s uses unsafe verifier workflow pattern %q", file, forbidden)
			}
		}
		for _, want := range []string{
			"verify-material:",
			"verify-acceptance:",
			"verify-gate:",
			"needs: [verify-material, verify-acceptance]",
			"ref: ${{ github.event.pull_request.base.sha || github.sha }}",
			"persist-credentials: false",
			"Fetch pull request receipt source",
			"if: github.event_name == 'pull_request_target'",
			"git fetch --no-tags pr-head",
			"SCAFLD_VERIFY_HEAD: ${{ github.event.pull_request.head.sha || github.sha }}",
			"SCAFLD_VERIFY_MODE: material",
			"SCAFLD_VERIFY_MODE: full",
			"if: ${{ always() && github.event_name == 'pull_request_target' }}",
			`needs.verify-material.result`,
			`needs.verify-acceptance.result`,
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing safe verifier workflow fragment %q", file, want)
			}
		}
	}
}

func TestVerifierScriptCanReadReceiptFromSeparateHead(t *testing.T) {
	t.Parallel()

	for _, file := range []string{
		filepath.Join("..", "..", "scripts", "scafld-verify.sh"),
		filepath.Join("..", "..", "internal", "adapters", "corebundle", "assets", "initwire", "scripts", "scafld-verify.sh"),
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, want := range []string{
			`head_ref="${SCAFLD_VERIFY_HEAD:-HEAD}"`,
			`git diff --name-only "$target" "$head_ref"`,
			`git rev-parse "$head_ref^{commit}"`,
			`git show "$head_ref:$receipt"`,
			`git worktree add --detach "$acceptance_root" "$resolved_head"`,
			`--material-ref "$resolved_head"`,
			`--material-only`,
			`--acceptance-root "$acceptance_root"`,
			`mode="${SCAFLD_VERIFY_MODE:-full}"`,
			`SCAFLD_VERIFY_SANITIZED=1`,
			`exec env -i`,
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing separate verify head support %q", file, want)
			}
		}
	}
}

func TestVerifierScriptUsesFetchedHeadReceiptAndMaterialRef(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	for _, file := range []string{
		filepath.Join("..", "..", "scripts", "scafld-verify.sh"),
		filepath.Join("..", "..", "internal", "adapters", "corebundle", "assets", "initwire", "scripts", "scafld-verify.sh"),
	} {
		file := file
		t.Run(filepath.ToSlash(file), func(t *testing.T) {
			t.Parallel()
			script, err := filepath.Abs(file)
			if err != nil {
				t.Fatal(err)
			}

			root := t.TempDir()
			releaseGitCmd(t, root, "init")
			releaseGitCmd(t, root, "config", "user.name", "scafld")
			releaseGitCmd(t, root, "config", "user.email", "scafld@example.invalid")
			writeReleaseFile(t, root, ".scafld/trusted-keys.json", `{"version":"scafld.trusted_keys.v1","keys":[]}`+"\n")
			writeReleaseFile(t, root, ".scafld/receipts/task.json", "base receipt\n")
			writeReleaseFile(t, root, "app.txt", "base\n")
			releaseGitCmd(t, root, "add", ".")
			releaseGitCmd(t, root, "commit", "-m", "base")
			base := releaseGitCmd(t, root, "rev-parse", "HEAD")

			writeReleaseFile(t, root, ".scafld/receipts/task.json", "head receipt\n")
			writeReleaseFile(t, root, "app.txt", "head\n")
			releaseGitCmd(t, root, "add", ".")
			releaseGitCmd(t, root, "commit", "-m", "head")
			head := releaseGitCmd(t, root, "rev-parse", "HEAD")
			releaseGitCmd(t, root, "checkout", base)

			fakeBin := filepath.Join(t.TempDir(), "bin")
			if err := os.MkdirAll(fakeBin, 0o755); err != nil {
				t.Fatal(err)
			}
			fakeScafld := filepath.Join(fakeBin, "scafld")
			fakeScript := `#!/usr/bin/env sh
	set -eu
	test -z "${SECRET_TOKEN:-}"
	test -z "${GITHUB_TOKEN:-}"
	test -z "${ACTIONS_RUNTIME_TOKEN:-}"
	test -z "${GH_TOKEN:-}"
		if [ "${SCAFLD_VERIFY_MODE:-full}" = "full" ]; then
		  test "${SCAFLD_VERIFY_SANITIZED:-}" = "1"
		fi
		if [ -e /proc/$PPID/environ ]; then
		  ! tr '\000' '\n' < /proc/$PPID/environ | grep -E '^(SECRET_TOKEN|GITHUB_TOKEN|ACTIONS_RUNTIME_TOKEN|GH_TOKEN)='
		fi
		printf '%s\n' "$@" > "$CAPTURE_ARGS"
		cat "$2" > "$CAPTURE_RECEIPT"
		`
			if err := os.WriteFile(fakeScafld, []byte(fakeScript), 0o755); err != nil {
				t.Fatal(err)
			}

			captureDir := t.TempDir()
			argsPath := filepath.Join(captureDir, "args.txt")
			receiptPath := filepath.Join(captureDir, "receipt.txt")
			cmd := exec.Command("sh", script)
			cmd.Dir = root
			cmd.Env = append(os.Environ(),
				"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"SCAFLD_VERIFY_TARGET="+base,
				"SCAFLD_VERIFY_HEAD="+head,
				"SCAFLD_RECEIPT_PATH=",
				"SCAFLD_TRUSTED_KEYS=",
				"RUNNER_TEMP="+t.TempDir(),
				"CAPTURE_ARGS="+argsPath,
				"CAPTURE_RECEIPT="+receiptPath,
				"SECRET_TOKEN=leak",
				"GITHUB_TOKEN=leak",
				"ACTIONS_RUNTIME_TOKEN=leak",
				"GH_TOKEN=leak",
				"CI=true",
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s failed: %v\n%s", file, err, out)
			}

			receiptData, err := os.ReadFile(receiptPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(receiptData) != "head receipt\n" {
				t.Fatalf("script passed stale receipt content %q, want head receipt", receiptData)
			}
			argsData, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
			if len(args) == 0 || args[0] != "verify" {
				t.Fatalf("unexpected scafld args: %q", argsData)
			}
			if got, ok := releaseArgValue(args, "--target"); !ok || got != base {
				t.Fatalf("target arg = %q ok=%v, want base %s; args=%q", got, ok, base, argsData)
			}
			if got, ok := releaseArgValue(args, "--material-ref"); !ok || got != head {
				t.Fatalf("material-ref arg = %q ok=%v, want head %s; args=%q", got, ok, head, argsData)
			}
			if got, ok := releaseArgValue(args, "--acceptance-root"); !ok || strings.TrimSpace(got) == "" {
				t.Fatalf("acceptance-root arg = %q ok=%v, want fetched-head worktree; args=%q", got, ok, argsData)
			}

			materialArgsPath := filepath.Join(captureDir, "material-args.txt")
			materialReceiptPath := filepath.Join(captureDir, "material-receipt.txt")
			materialCmd := exec.Command("sh", script)
			materialCmd.Dir = root
			materialCmd.Env = append(os.Environ(),
				"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"SCAFLD_VERIFY_TARGET="+base,
				"SCAFLD_VERIFY_HEAD="+head,
				"SCAFLD_VERIFY_MODE=material",
				"SCAFLD_RECEIPT_PATH=",
				"SCAFLD_TRUSTED_KEYS=",
				"RUNNER_TEMP="+t.TempDir(),
				"CAPTURE_ARGS="+materialArgsPath,
				"CAPTURE_RECEIPT="+materialReceiptPath,
				"CI=true",
			)
			if out, err := materialCmd.CombinedOutput(); err != nil {
				t.Fatalf("%s material mode failed: %v\n%s", file, err, out)
			}
			materialArgsData, err := os.ReadFile(materialArgsPath)
			if err != nil {
				t.Fatal(err)
			}
			materialArgs := strings.Split(strings.TrimSpace(string(materialArgsData)), "\n")
			if _, ok := releaseArgValue(materialArgs, "--acceptance-root"); ok {
				t.Fatalf("material mode passed acceptance-root; args=%q", materialArgsData)
			}
			if !releaseHasArg(materialArgs, "--material-only") {
				t.Fatalf("material mode args missing --material-only: %q", materialArgsData)
			}
		})
	}
}

func mustMatchVersion(t *testing.T, text string, pattern string) string {
	t.Helper()
	match := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if len(match) != 2 {
		t.Fatalf("version pattern %q not found", pattern)
	}
	return match[1]
}

func writeReleaseFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func releaseGitCmd(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func releaseArgValue(args []string, flag string) (string, bool) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

func releaseHasArg(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func TestReleaseWorkflowPublishesRegistryPackages(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"softprops/action-gh-release",
		"PYPI_API_TOKEN",
		"NPM_TOKEN",
		"npm publish --access public",
		"pypa/gh-action-pypi-publish",
		"scripts/build-release-artifacts.sh",
		"scripts/smoke-release-installers.sh",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("release workflow missing %q", want)
		}
	}
}

func TestReleaseScriptBuildsOptimizedRawBinaries(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "build-release-artifacts.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"CGO_ENABLED=0",
		"-mod=readonly",
		"-trimpath",
		"-buildvcs=false",
		"-gcflags=all=-l",
		"-s -w -buildid=",
		"-X github.com/nilstate/scafld/v2/internal/adapters/cli.version=",
		"scripts/check-release-size.sh",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("release build script missing %q", want)
		}
	}
	if strings.Contains(strings.ToLower(text), "upx") {
		t.Fatal("release build must not depend on executable packers")
	}
}

func TestReleaseInstallerSmokeUsesLocalDist(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "smoke-release-installers.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"SCAFLD_INSTALL_BASE_URL",
		"Path(os.environ[\"DIST\"]).resolve().as_uri()",
		"node \"$tmp/npm/lib/install.js\"",
		"node \"$tmp/npm/bin/scafld.js\" --version",
		"from scafld_launcher.cli import main",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("installer smoke missing %q", want)
		}
	}
}

func TestNpmInstallerSupportsLocalReleaseAssets(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "package", "npm", "lib", "install.js"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`require("node:http")`, "fileURLToPath", `parsed.protocol === "file:"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("npm installer missing local asset support %q", want)
		}
	}
}

func TestPackageLaunchersVerifyChecksums(t *testing.T) {
	t.Parallel()
	files := []string{
		filepath.Join("..", "..", "package", "npm", "lib", "install.js"),
		filepath.Join("..", "..", "package", "pypi", "src", "scafld_launcher", "install.py"),
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "checksums.txt") || !strings.Contains(strings.ToLower(text), "sha256") {
			t.Fatalf("%s does not verify release checksums", file)
		}
	}
}

func TestRegistryTemplatesPointAtPrimaryReleaseAssets(t *testing.T) {
	t.Parallel()
	files := []string{
		filepath.Join("..", "..", "package", "homebrew", "scafld.rb.tmpl"),
		filepath.Join("..", "..", "package", "scoop", "scafld.json.tmpl"),
		filepath.Join("..", "..", "package", "winget", "scafld.installer.yaml.tmpl"),
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "github.com/nilstate/scafld/releases/download/v{{VERSION}}") {
			t.Fatalf("%s does not use primary release assets", file)
		}
	}
}

func TestRegistryTemplatesUsePublicPackageIdentity(t *testing.T) {
	t.Parallel()

	homebrew, err := os.ReadFile(filepath.Join("..", "..", "package", "homebrew", "scafld.rb.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	homebrewText := string(homebrew)
	for _, want := range []string{`license "MIT"`, `chmod 0755, bin/"scafld"`} {
		if !strings.Contains(homebrewText, want) {
			t.Fatalf("homebrew template missing %q", want)
		}
	}

	for _, file := range []string{
		filepath.Join("..", "..", "package", "winget", "scafld.installer.yaml.tmpl"),
		filepath.Join("..", "..", "package", "winget", "scafld.version.yaml.tmpl"),
		filepath.Join("..", "..", "package", "winget", "scafld.yaml.tmpl"),
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "PackageIdentifier: 0state.scafld") {
			t.Fatalf("%s must use the public 0state package identifier", file)
		}
		if strings.Contains(text, "Nilstate.Scafld") {
			t.Fatalf("%s must not expose the GitHub org as package identity", file)
		}
		if !strings.HasPrefix(text, "# yaml-language-server: $schema=https://aka.ms/winget-manifest.") {
			t.Fatalf("%s must start with the Winget schema header", file)
		}
	}
}
