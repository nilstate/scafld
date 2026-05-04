package golden

import (
	"os"
	"testing"
)

func UpdateEnabled() bool {
	return os.Getenv("SCAFLD_UPDATE_GOLDEN") == "1"
}

func Assert(t *testing.T, path string, got []byte) {
	t.Helper()
	if UpdateEnabled() {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch for %s\nwant:\n%s\n\ngot:\n%s", path, want, got)
	}
}
