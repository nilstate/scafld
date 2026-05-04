package ci

import (
	"os/exec"
	"testing"
)

func TestLocalCI(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("make", "fmt")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make fmt: %v\n%s", err, out)
	}
}
