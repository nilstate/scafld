// Package reviewmaterial projects the task-owned material surface shared by
// status, handoff, and API-style adapters.
package reviewmaterial

import (
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/reviewgate"
	"github.com/nilstate/scafld/v2/internal/core/reviewscope"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	coreworkspace "github.com/nilstate/scafld/v2/internal/core/workspace"
)

const (
	StatusUnchanged                   = "unchanged"
	StatusChanged                     = "changed"
	StatusCurrentUnavailable          = "current_unavailable"
	StatusReviewedMaterialUnavailable = "reviewed_material_unavailable"
	StatusUnreviewed                  = "unreviewed"
)

// Input is the complete read-only material projection input.
type Input struct {
	Model                    spec.Model
	Ledger                   session.Session
	CurrentSnapshot          []string
	HasCurrentSnapshot       bool
	Authority                reviewgate.Authority
	CurrentMaterialDigest    string
	HasCurrentMaterialDigest bool
}

// Projection describes the task-owned workspace surface and its review freshness.
type Projection struct {
	Scope                  []string `json:"scope,omitempty"`
	BaselinePaths          []string `json:"baseline_paths,omitempty"`
	TaskChanges            []string `json:"task_changes,omitempty"`
	AmbientDrift           []string `json:"ambient_drift,omitempty"`
	ReviewedScope          []string `json:"reviewed_scope,omitempty"`
	ReviewedMaterialDigest string   `json:"reviewed_material_digest,omitempty"`
	CurrentMaterialDigest  string   `json:"current_material_digest,omitempty"`
	MaterialStatus         string   `json:"material_status,omitempty"`
}

// Project derives the shared task-material read model.
func Project(input Input) Projection {
	baseline := baselineSnapshot(input.Ledger)
	projection := reviewscope.Project(input.Model, nil, baseline, input.CurrentSnapshot)
	if !input.HasCurrentSnapshot {
		scope := reviewscope.Derive(input.Model, nil, reviewevidence.ComparisonSnapshot(baseline))
		projection = reviewscope.Projection{
			Scope:    scope,
			Baseline: coreworkspace.Filter(reviewevidence.ComparisonSnapshot(baseline), scope),
		}
	}

	out := Projection{
		Scope:         append([]string(nil), projection.Scope...),
		BaselinePaths: coreworkspace.Paths(projection.Baseline),
		TaskChanges:   coreworkspace.MutationStrings(projection.TaskChanges),
		AmbientDrift:  coreworkspace.MutationStrings(projection.AmbientDrift),
	}
	if input.Authority.Valid && input.Authority.ReviewEntry.Type == "review" {
		out.ReviewedScope = append([]string(nil), input.Authority.ReviewEntry.ReviewedScope...)
		out.ReviewedMaterialDigest = strings.TrimSpace(input.Authority.ReviewEntry.ReviewedMaterialDigest)
	}
	if input.HasCurrentMaterialDigest {
		out.CurrentMaterialDigest = strings.TrimSpace(input.CurrentMaterialDigest)
	}
	out.MaterialStatus = materialStatus(input.Authority, out)
	return out
}

// Empty reports whether the projection has no useful material evidence.
func (p Projection) Empty() bool {
	return len(p.Scope) == 0 &&
		len(p.BaselinePaths) == 0 &&
		len(p.TaskChanges) == 0 &&
		len(p.AmbientDrift) == 0 &&
		len(p.ReviewedScope) == 0 &&
		p.ReviewedMaterialDigest == "" &&
		p.CurrentMaterialDigest == "" &&
		p.MaterialStatus == ""
}

func baselineSnapshot(ledger session.Session) []string {
	entry, ok := session.FirstWorkspaceBaseline(ledger)
	if !ok {
		return nil
	}
	return session.WorkspaceBaselineSnapshot(entry)
}

func materialStatus(authority reviewgate.Authority, projection Projection) string {
	switch {
	case projection.ReviewedMaterialDigest != "" && projection.CurrentMaterialDigest != "":
		if projection.ReviewedMaterialDigest == projection.CurrentMaterialDigest {
			return StatusUnchanged
		}
		return StatusChanged
	case projection.ReviewedMaterialDigest != "":
		return StatusCurrentUnavailable
	case authority.Valid && authority.Found:
		return StatusReviewedMaterialUnavailable
	case len(projection.Scope) > 0 || len(projection.TaskChanges) > 0 || len(projection.AmbientDrift) > 0:
		return StatusUnreviewed
	default:
		return ""
	}
}
