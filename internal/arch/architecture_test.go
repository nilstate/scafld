package arch

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const modulePath = "github.com/nilstate/scafld/v2"

type goPackage struct {
	ImportPath string
	Imports    []string
	Deps       []string
	Standard   bool
}

func TestImportBoundaries(t *testing.T) {
	pkgs := listPackages(t, "./...")
	for _, pkg := range pkgs {
		checkPackageImports(t, pkg)
	}
}

func TestCoreIsPure(t *testing.T) {
	pkgs := listPackages(t, "./internal/core/...")
	for _, pkg := range pkgs {
		for _, imp := range pkg.Imports {
			if isStdlib(imp) || strings.HasPrefix(imp, modulePath+"/internal/core/") {
				continue
			}
			t.Fatalf("%s imports %s; core may import only stdlib and core packages", pkg.ImportPath, imp)
		}
	}
}

func TestCoreTransitiveDepsAreStdlib(t *testing.T) {
	deps := listPackages(t, "-deps", "./internal/core/...")
	for _, dep := range deps {
		if dep.Standard || strings.HasPrefix(dep.ImportPath, modulePath+"/internal/core") {
			continue
		}
		t.Fatalf("core has non-stdlib transitive dependency %s", dep.ImportPath)
	}
}

func TestAppDoesNotImportAdapters(t *testing.T) {
	pkgs := listPackages(t, "./internal/app/...")
	for _, pkg := range pkgs {
		for _, imp := range pkg.Imports {
			if strings.Contains(imp, "/internal/adapters/") || strings.Contains(imp, "/internal/platform/") {
				t.Fatalf("%s imports outward dependency %s", pkg.ImportPath, imp)
			}
		}
	}
}

func TestPortsAreUseCaseOwned(t *testing.T) {
	if _, err := os.Stat(filepath.Join(repoRoot(t), "internal", "ports")); err == nil {
		t.Fatal("internal/ports must not exist; ports are owned by use-case packages")
	}
}

func TestPortsAreNarrow(t *testing.T) {
	root := filepath.Join(repoRoot(t), "internal", "app")
	const maxMethods = 3
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			iface, ok := spec.Type.(*ast.InterfaceType)
			if !ok {
				return true
			}
			if len(iface.Methods.List) > maxMethods {
				t.Fatalf("%s: interface %s has %d methods; max %d", path, spec.Name.Name, len(iface.Methods.List), maxMethods)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestProviderBoundary(t *testing.T) {
	pkgs := append(listPackages(t, "./internal/core/..."), listPackages(t, "./internal/app/...")...)
	for _, pkg := range pkgs {
		for _, imp := range pkg.Imports {
			if strings.Contains(imp, "/internal/adapters/providers") {
				t.Fatalf("%s imports provider implementation %s", pkg.ImportPath, imp)
			}
		}
	}
}

func TestCLIIsThin(t *testing.T) {
	cliDir := filepath.Join(repoRoot(t), "internal", "adapters", "cli")
	const maxLines = 700
	total := 0
	err := filepath.WalkDir(cliDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != cliDir {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, "_test.go") || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		total += bytes.Count(data, []byte("\n"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if total > maxLines {
		t.Fatalf("CLI adapter has %d production lines; keep command handlers thin", total)
	}
}

func checkPackageImports(t *testing.T, pkg goPackage) {
	t.Helper()
	path := pkg.ImportPath
	for _, imp := range pkg.Imports {
		switch {
		case strings.Contains(path, "/internal/core/"):
			if !isStdlib(imp) && !strings.HasPrefix(imp, modulePath+"/internal/core/") {
				t.Fatalf("%s imports %s; core must stay pure", path, imp)
			}
		case strings.Contains(path, "/internal/app/"):
			if strings.Contains(imp, "/internal/adapters/") || strings.Contains(imp, "/internal/platform/") {
				t.Fatalf("%s imports outward dependency %s", path, imp)
			}
		case strings.Contains(path, "/internal/platform/"):
			if !isStdlib(imp) {
				t.Fatalf("%s imports %s; platform must stay product-policy free", path, imp)
			}
		case strings.Contains(path, "/internal/adapters/") && !strings.Contains(path, "/internal/adapters/cli"):
			if strings.Contains(imp, "/internal/app/") || strings.Contains(imp, "/internal/adapters/") {
				t.Fatalf("%s imports %s; non-CLI adapters must not compose app or other adapters", path, imp)
			}
		}
	}
}

func listPackages(t *testing.T, args ...string) []goPackage {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list", "-json"}, args...)...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list %v: %v\n%s", args, err, out)
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	var pkgs []goPackage
	for {
		var pkg goPackage
		if err := dec.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode go list output: %v", err)
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

func isStdlib(path string) bool {
	return !strings.Contains(path, ".") && !strings.HasPrefix(path, modulePath)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("go.mod not found")
		}
		wd = parent
	}
}
