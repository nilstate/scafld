package harden

import (
	"fmt"
	"strings"

	coreharden "github.com/nilstate/scafld/v2/internal/core/harden"
	"github.com/nilstate/scafld/v2/internal/core/reviewcontext"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

func hardenContextPacket(model spec.Model, specPath string, prompt string) reviewcontext.Packet {
	sourcePath := strings.TrimSpace(specPath)
	if sourcePath == "" {
		sourcePath = model.TaskID
	}
	sections := []reviewcontext.Section{
		hardenSection("task_contract", "Draft Task Contract", 10, hardenTaskContractBody(model), "spec", sourcePath),
		hardenSection("scope", "Scope And Touchpoints", 20, hardenScopeBody(model), "spec", sourcePath),
		hardenSection("phases", "Planned Phases", 30, hardenPhasesBody(model), "spec", sourcePath),
		hardenSection("acceptance", "Acceptance And Rollback", 40, hardenAcceptanceBody(model), "spec", sourcePath),
		hardenSection("harden_contract", "Harden Contract", 50, prompt, "scafld", "harden"),
		hardenSection("provider_instruction", "Provider Instruction", 90, hardenProviderInstructionBody(), "scafld", "harden"),
	}
	return reviewcontext.Packet{TaskID: model.TaskID, Title: model.Title, Status: string(model.Status), Sections: sections}
}

func hardenSection(key string, title string, order int, body string, kind string, path string) reviewcontext.Section {
	body = strings.TrimSpace(body)
	sourcePath := strings.TrimSpace(path)
	if sourcePath == "" {
		sourcePath = key
	}
	if !strings.Contains(sourcePath, "#") {
		sourcePath += "#" + key
	}
	return reviewcontext.Section{
		Key:     key,
		Title:   title,
		Order:   order,
		Body:    body,
		Sources: []reviewcontext.Source{reviewcontext.SourceForContent("derived_"+kind, sourcePath, []byte(body))},
	}
}

func hardenTaskContractBody(model spec.Model) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Title: %s\nStatus: %s\nSize: %s\nRisk: %s\n", model.Title, model.Status, model.Size, model.RiskLevel)
	if strings.TrimSpace(model.Summary) != "" {
		fmt.Fprintf(&b, "\nSummary:\n%s\n", strings.TrimSpace(model.Summary))
	}
	if len(model.Objectives) > 0 {
		b.WriteString("\nObjectives:\n")
		for _, objective := range model.Objectives {
			fmt.Fprintf(&b, "- %s\n", objective)
		}
	}
	if len(model.Scope) > 0 {
		b.WriteString("\nScope:\n")
		for _, item := range model.Scope {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	return b.String()
}

func hardenScopeBody(model spec.Model) string {
	var b strings.Builder
	if len(model.Context.Packages) > 0 {
		b.WriteString("Packages:\n")
		for _, item := range model.Context.Packages {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	if len(model.Context.FilesImpacted) > 0 {
		b.WriteString("\nFiles impacted:\n")
		for _, item := range model.Context.FilesImpacted {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	if len(model.Context.Invariants) > 0 {
		b.WriteString("\nInvariants:\n")
		for _, item := range model.Context.Invariants {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	if len(model.Touchpoints) > 0 {
		b.WriteString("\nTouchpoints:\n")
		for _, item := range model.Touchpoints {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	if len(model.Risks) > 0 {
		b.WriteString("\nRisks:\n")
		for _, risk := range model.Risks {
			fmt.Fprintf(&b, "- %s\n", risk.Description)
			if strings.TrimSpace(risk.Mitigation) != "" {
				fmt.Fprintf(&b, "  - Mitigation: %s\n", risk.Mitigation)
			}
		}
	}
	return b.String()
}

func hardenPhasesBody(model spec.Model) string {
	var b strings.Builder
	for _, phase := range model.Phases {
		fmt.Fprintf(&b, "Phase %d `%s`: %s\n", phase.Number, phase.ID, phase.Name)
		if strings.TrimSpace(phase.Objective) != "" {
			fmt.Fprintf(&b, "- Objective: %s\n", phase.Objective)
		}
		if len(phase.Dependencies) > 0 {
			fmt.Fprintf(&b, "- Dependencies: %s\n", strings.Join(phase.Dependencies, ", "))
		}
		if len(phase.Changes) > 0 {
			b.WriteString("- Changes:\n")
			for _, change := range phase.Changes {
				fmt.Fprintf(&b, "  - %s\n", change)
			}
		}
		for _, criterion := range phase.Acceptance {
			fmt.Fprintf(&b, "- Acceptance `%s`: %s\n", criterion.ID, criterion.Command)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func hardenAcceptanceBody(model spec.Model) string {
	var b strings.Builder
	if model.Acceptance.ValidationProfile != "" {
		fmt.Fprintf(&b, "Validation profile: %s\n\n", model.Acceptance.ValidationProfile)
	}
	if len(model.Acceptance.Criteria) > 0 {
		b.WriteString("Criteria:\n")
		for _, criterion := range model.Acceptance.Criteria {
			fmt.Fprintf(&b, "- `%s` %s: %s\n", criterion.ID, criterion.ExpectedKind, criterion.Command)
		}
	}
	if len(model.Rollback) > 0 {
		b.WriteString("\nRollback:\n")
		for _, item := range model.Rollback {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	return b.String()
}

func hardenProviderInstructionBody() string {
	return "Harden mode is read-only. Do not edit files. Challenge design and scope before path and command bookkeeping: ask whether the plan has a right to exist, what root product or system problem it solves, what shared core/app contract should own the behavior, and whether API, MCP, CLI, provider, and docs surfaces stay light adapters instead of drifting into separate implementations. Then verify declared paths and commands exist or are intentionally future files, question migration and cutover claims, test whether acceptance commands can run at the right phase, and verify rollback/repair is credible. Find as many real spec issues as the round budget allows, but do not pad the dossier with weak or speculative observations. The HardenDossier `observations` array is a fixed six-dimension ledger in this order: " + coreharden.RequiredDimensionList() + ". Include at least one entry for each dimension and fill `result` and `anchor` for every entry. Use result `clean`, `advisory`, `blocks`, or `n/a`; non-clean observations require a note, and question-shaped observations can include a default answer. Use status `open` only for unresolved blocking observations, and `fixed`, `accepted_risk`, or `superseded` when a blocking observation is already resolved. Do not write a verdict; scafld derives it from dimension coverage and unresolved blocking observations. Call `submit_harden` exactly once with the final HardenDossier; do not emit final prose or JSON text."
}
