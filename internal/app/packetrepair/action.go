package packetrepair

import (
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/hardengate"
	"github.com/nilstate/scafld/v2/internal/core/reviewgate"
)

func ReviewAction(taskID string, artifactPath string) gate.RepairAction {
	return action("ReviewDossier", artifactPath, reviewgate.RepairPacketCommand(taskID, artifactPath), "review")
}

func HardenAction(taskID string, artifactPath string) gate.RepairAction {
	return action("HardenDossier", artifactPath, hardengate.RepairPacketCommand(taskID, artifactPath), "harden")
}

func action(packetType string, artifactPath string, command string, gateName string) gate.RepairAction {
	packetType = strings.TrimSpace(packetType)
	return gate.RepairAction{
		ArtifactPath:     strings.TrimSpace(artifactPath),
		RequiredEdit:     "set repaired_packet to exactly one valid " + packetType + " JSON object copied or normalized from the rejected provider packet; do not invent, remove, soften, or rewrite findings",
		CommandAfterEdit: strings.TrimSpace(command),
		Fallback:         "only rerun external " + strings.TrimSpace(gateName) + " if the artifact or diagnostic contains no usable provider packet; fix provider availability/output first",
	}
}

func LooksLikeArtifactPath(gateName string, path string) bool {
	gateName = strings.TrimSpace(gateName)
	path = strings.TrimSpace(path)
	if gateName == "" || path == "" {
		return false
	}
	base := filepath.Base(path)
	return strings.HasPrefix(base, "provider-packet-repair-"+gateName+"-") && strings.HasSuffix(base, ".json")
}

func FollowUp(action gate.RepairAction) string {
	return strings.Join(Blockers(action), "; ")
}

func Blockers(action gate.RepairAction) []string {
	var out []string
	if action.ArtifactPath != "" {
		out = append(out, "read provider repair artifact: "+action.ArtifactPath)
	}
	if action.RequiredEdit != "" {
		out = append(out, action.RequiredEdit)
	}
	if action.CommandAfterEdit != "" {
		out = append(out, "then run: "+action.CommandAfterEdit)
	}
	if action.Fallback != "" {
		out = append(out, action.Fallback)
	}
	return out
}
