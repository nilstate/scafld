package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModulePathIsPrimaryRepository(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "module github.com/nilstate/scafld/v2\n") {
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
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("release workflow missing %q", want)
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
	}
}
