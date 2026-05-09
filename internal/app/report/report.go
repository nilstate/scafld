package report

import (
	"context"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec listing port used by reports.
type SpecStore interface {
	List(context.Context) ([]spec.Record, error)
}

// ArchiveAwareSpecStore can include archived terminal specs in reports.
type ArchiveAwareSpecStore interface {
	ListAll(context.Context) ([]spec.Record, error)
}

// SessionStore lists session ledgers used for evidence-derived metrics.
type SessionStore interface {
	List(context.Context) ([]session.Session, error)
}

// Output summarizes workspace task counts and session-derived product metrics.
type Output struct {
	Total    int                 `json:"total"`
	ByStatus map[spec.Status]int `json:"by_status"`
	Metrics  Metrics             `json:"metrics"`
}

// Metrics captures the reporting surface scafld uses to judge protocol quality.
type Metrics struct {
	FirstAttemptPassRate      float64        `json:"first_attempt_pass_rate"`
	FirstAttemptPasses        int            `json:"first_attempt_passes"`
	FirstAttemptTotal         int            `json:"first_attempt_total"`
	RecoveryConvergenceRate   float64        `json:"recovery_convergence_rate"`
	RecoveredTasks            int            `json:"recovered_tasks"`
	RecoveryTotal             int            `json:"recovery_total"`
	ChallengeOverrideRate     float64        `json:"challenge_override_rate"`
	ChallengeOverrides        int            `json:"challenge_overrides"`
	ReviewChallengeTotal      int            `json:"review_challenge_total"`
	ReviewPassRate            float64        `json:"review_pass_rate"`
	ReviewPasses              int            `json:"review_passes"`
	ReviewTotal               int            `json:"review_total"`
	ReviewDossierCoverage     float64        `json:"review_dossier_coverage"`
	ReviewDossierTotal        int            `json:"review_dossier_total"`
	ReviewFindingsTotal       int            `json:"review_findings_total"`
	ReviewOpenBlockersTotal   int            `json:"review_open_blockers_total"`
	ReviewAttackAnglesTotal   int            `json:"review_attack_angles_total"`
	ReviewModeDistribution    map[string]int `json:"review_mode_distribution,omitempty"`
	WorkspaceBaselineCoverage float64        `json:"workspace_baseline_coverage"`
	WorkspaceBaselineTasks    int            `json:"workspace_baseline_tasks"`
	SessionTotal              int            `json:"session_total"`
	BlockedGateDistribution   map[string]int `json:"blocked_gate_distribution,omitempty"`
}

// Run aggregates spec records by status and session evidence by task.
func Run(ctx context.Context, store SpecStore, sessions SessionStore) (Output, error) {
	records, err := listRecords(ctx, store)
	if err != nil {
		return Output{}, err
	}
	out := Output{Total: len(records), ByStatus: map[spec.Status]int{}}
	for _, record := range records {
		out.ByStatus[record.Status]++
	}
	if sessions != nil {
		ledgers, err := sessions.List(ctx)
		if err != nil {
			return Output{}, err
		}
		out.Metrics = summarizeSessions(ledgers)
	}
	return out, nil
}

func listRecords(ctx context.Context, store SpecStore) ([]spec.Record, error) {
	if archiveAware, ok := store.(ArchiveAwareSpecStore); ok {
		return archiveAware.ListAll(ctx)
	}
	return store.List(ctx)
}

func summarizeSessions(ledgers []session.Session) Metrics {
	var metrics Metrics
	metrics.SessionTotal = len(ledgers)
	metrics.BlockedGateDistribution = map[string]int{}
	metrics.ReviewModeDistribution = map[string]int{}
	for _, ledger := range ledgers {
		if _, ok := session.FirstWorkspaceBaseline(ledger); ok {
			metrics.WorkspaceBaselineTasks++
		}
		recordBlockedGates(&metrics, ledger)
		summary := summarizeLedger(ledger)
		if summary.firstBuildResult != "" {
			metrics.FirstAttemptTotal++
			if summary.firstBuildResult == string(spec.StatusReview) {
				metrics.FirstAttemptPasses++
			}
		}
		if summary.firstBuildResult == string(spec.StatusBlocked) {
			metrics.RecoveryTotal++
			if summary.recovered {
				metrics.RecoveredTasks++
			}
		}
		if summary.reviewTotal > 0 {
			metrics.ReviewTotal += summary.reviewTotal
			metrics.ReviewPasses += summary.reviewPasses
		}
		metrics.ReviewDossierTotal += summary.reviewDossierTotal
		metrics.ReviewFindingsTotal += summary.reviewFindingsTotal
		metrics.ReviewOpenBlockersTotal += summary.reviewOpenBlockersTotal
		metrics.ReviewAttackAnglesTotal += summary.reviewAttackAnglesTotal
		for mode, count := range summary.reviewModeDistribution {
			metrics.ReviewModeDistribution[mode] += count
		}
		if summary.hadReviewChallenge {
			metrics.ReviewChallengeTotal++
			if summary.override {
				metrics.ChallengeOverrides++
			}
		}
	}
	metrics.FirstAttemptPassRate = ratio(metrics.FirstAttemptPasses, metrics.FirstAttemptTotal)
	metrics.RecoveryConvergenceRate = ratio(metrics.RecoveredTasks, metrics.RecoveryTotal)
	metrics.ReviewPassRate = ratio(metrics.ReviewPasses, metrics.ReviewTotal)
	metrics.ReviewDossierCoverage = ratio(metrics.ReviewDossierTotal, metrics.ReviewTotal)
	metrics.ChallengeOverrideRate = ratio(metrics.ChallengeOverrides, metrics.ReviewChallengeTotal)
	metrics.WorkspaceBaselineCoverage = ratio(metrics.WorkspaceBaselineTasks, metrics.SessionTotal)
	return metrics
}

func recordBlockedGates(metrics *Metrics, ledger session.Session) {
	for _, entry := range ledger.Entries {
		switch entry.Type {
		case "build":
			if entry.Status == string(spec.StatusBlocked) {
				metrics.BlockedGateDistribution["build"]++
			}
		case "review":
			if entry.Status == corereview.VerdictFail {
				metrics.BlockedGateDistribution["review"]++
			}
		case "review_attempt":
			if entry.Status == "failed" {
				metrics.BlockedGateDistribution["review_provider"]++
			}
		}
	}
}

type ledgerSummary struct {
	firstBuildResult        string
	recovered               bool
	reviewPasses            int
	reviewTotal             int
	reviewDossierTotal      int
	reviewFindingsTotal     int
	reviewOpenBlockersTotal int
	reviewAttackAnglesTotal int
	reviewModeDistribution  map[string]int
	hadReviewChallenge      bool
	override                bool
}

func summarizeLedger(ledger session.Session) ledgerSummary {
	summary := ledgerSummary{reviewModeDistribution: map[string]int{}}
	latestReviewStatus := ""
	latestReviewProvider := ""
	for _, entry := range ledger.Entries {
		switch entry.Type {
		case "build":
			if entry.Status == "active" || entry.Status == "" {
				continue
			}
			if summary.firstBuildResult == "" {
				summary.firstBuildResult = entry.Status
			}
			if entry.Status == string(spec.StatusReview) || entry.Status == string(spec.StatusCompleted) {
				summary.recovered = true
			}
		case "review":
			summary.reviewTotal++
			latestReviewStatus = entry.Status
			latestReviewProvider = entry.Provider
			if dossier, ok := corereview.DecodeDossier(entry.Output); ok {
				summary.reviewDossierTotal++
				summary.reviewFindingsTotal += len(dossier.Findings)
				summary.reviewOpenBlockersTotal += corereview.OpenBlockerCount(dossier.Findings)
				summary.reviewAttackAnglesTotal += len(dossier.AttackLog)
				if dossier.Mode != "" {
					summary.reviewModeDistribution[string(dossier.Mode)]++
				}
			}
			if entry.Status == "pass" {
				summary.reviewPasses++
			}
			if entry.Status == "fail" {
				summary.hadReviewChallenge = true
			}
			if entry.Provider == "human" {
				summary.hadReviewChallenge = true
				summary.override = true
			}
		case "review_override":
			summary.hadReviewChallenge = true
			summary.override = true
		case "complete":
			if summary.hadReviewChallenge && (latestReviewStatus != "pass" || !corereview.ValidCompletionProvider(latestReviewProvider)) {
				summary.override = true
			}
		}
	}
	return summary
}

func ratio(numerator int, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
