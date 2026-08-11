package packetrepair

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/providerpacket"
)

func TestWriteArtifactRequiresRejectedPacketText(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if path, ok := WriteArtifact(ArtifactInput{
		Root:   root,
		TaskID: "task",
		Gate:   "review",
		Name:   "provider-packet-repair-review-attempt",
		Source: providerpacket.Source{DiagnosticPath: "/tmp/provider.txt", Error: "provider produced no submission"},
		Err:    errors.New("provider produced no submission"),
	}); ok || path != "" {
		t.Fatalf("diagnostic-only provider failure wrote repair artifact: path=%q ok=%v", path, ok)
	}

	if path, ok := WriteArtifact(ArtifactInput{
		Root:   root,
		TaskID: "task",
		Gate:   "review",
		Name:   "provider-packet-repair-review-attempt",
		Source: providerpacket.Source{RejectedText: `{"bad":true}`, Error: "invalid packet"},
		Err:    errors.New("invalid packet"),
	}); !ok || path == "" {
		t.Fatalf("rejected packet text did not write repair artifact: path=%q ok=%v", path, ok)
	} else if want := filepath.Join(root, ".scafld", "runs", "task", "artifacts", "provider-packets", "provider-packet-repair-review-attempt.json"); path != want {
		t.Fatalf("repair path = %q, want %q", path, want)
	}
}
