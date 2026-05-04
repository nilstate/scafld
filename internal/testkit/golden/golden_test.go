package golden

import "testing"

func TestUpdateDisabledByDefault(t *testing.T) {
	if UpdateEnabled() {
		t.Fatal("golden updates must be opt-in")
	}
}
