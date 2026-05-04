package release

import (
	"os/exec"
	"testing"
)

func TestBuildMatrix(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "test", "./cmd/scafld")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build matrix smoke: %v\n%s", err, out)
	}
}
