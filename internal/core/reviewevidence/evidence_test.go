package reviewevidence

import "testing"

func TestMaterialDigestIgnoresStatusButTracksContent(t *testing.T) {
	t.Parallel()

	scope := []string{"api"}
	dirty := MaterialDigest(scope, []MaterialFile{{Path: "api/handler.go", SHA256: "one"}})
	committed := MaterialDigest(scope, []MaterialFile{{Path: "api/handler.go", SHA256: "one"}})
	if dirty != committed {
		t.Fatalf("same material digest changed across status transition: %s != %s", dirty, committed)
	}
	changed := MaterialDigest(scope, []MaterialFile{{Path: "api/handler.go", SHA256: "two"}})
	if dirty == changed {
		t.Fatalf("material digest did not change when file content changed: %s", dirty)
	}
}
