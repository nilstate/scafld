package markdown

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
)

var (
	ErrSpecNotFound      = errors.New("spec not found")
	ErrMalformedMarkdown = errors.New("malformed markdown spec")
	phaseHeadingPattern  = regexp.MustCompile(`^## Phase ([0-9]+):\s*(.+?)\s*$`)
	criterionLinePattern = regexp.MustCompile(`^- \[[ xX]\] ` + "`" + `([^` + "`" + `]+)` + "`" + `\s+([^-]+?)\s+-\s*(.*)$`)
	commandLinePattern   = regexp.MustCompile(`^\s+- Command:\s*` + "`" + `(.*)` + "`" + `\s*$`)
	expectedLinePattern  = regexp.MustCompile(`^\s+- Expected kind:\s*` + "`" + `?([^` + "`" + `\s]+)` + "`" + `?\s*$`)
	statusLinePattern    = regexp.MustCompile(`^\s+- Status:\s*([a-z_]+)\s*$`)
	evidenceLinePattern  = regexp.MustCompile(`^\s+- Evidence:\s*(.*)\s*$`)
	sourceLinePattern    = regexp.MustCompile(`^\s+- Source event:\s*(.*)\s*$`)
)

type Store struct {
	Root string
}

func (s Store) CreateDraft(ctx context.Context, model spec.Model) (string, error) {
	root := s.Root
	if root == "" {
		root = "."
	}
	path := filepath.Join(root, ".scafld", "specs", "drafts", model.TaskID+".md")
	return path, writeSpec(ctx, path, model)
}

func (s Store) Save(ctx context.Context, path string, model spec.Model) error {
	return writeSpec(ctx, path, model)
}

func (s Store) Load(ctx context.Context, taskID string) (spec.Model, string, error) {
	if err := ctx.Err(); err != nil {
		return spec.Model{}, "", err
	}
	path, err := s.Find(taskID)
	if err != nil {
		return spec.Model{}, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return spec.Model{}, "", fmt.Errorf("read spec %s: %w", path, err)
	}
	model, err := Parse(data)
	if err != nil {
		return spec.Model{}, "", err
	}
	return model, path, nil
}

func (s Store) Find(taskID string) (string, error) {
	root := s.Root
	if root == "" {
		root = "."
	}
	for _, dir := range []string{"drafts", "approved", "active", "archive"} {
		base := filepath.Join(root, ".scafld", "specs", dir)
		if dir == "archive" {
			var found string
			err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() || filepath.Base(path) != taskID+".md" {
					return nil
				}
				found = path
				return filepath.SkipAll
			})
			if err == nil && found != "" {
				return found, nil
			}
			continue
		}
		path := filepath.Join(base, taskID+".md")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrSpecNotFound, taskID)
}

func (s Store) List(ctx context.Context) ([]spec.Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root := s.Root
	if root == "" {
		root = "."
	}
	var records []spec.Record
	for _, dir := range []string{"drafts", "approved", "active"} {
		pattern := filepath.Join(root, ".scafld", "specs", dir, "*.md")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, path := range matches {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			model, err := Parse(data)
			if err != nil {
				return nil, err
			}
			records = append(records, spec.Record{TaskID: model.TaskID, Status: model.Status, Path: path, Title: model.Title})
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].TaskID < records[j].TaskID })
	return records, nil
}

func Parse(data []byte) (spec.Model, error) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return spec.Model{}, fmt.Errorf("%w: front matter is required", ErrMalformedMarkdown)
	}
	lines := splitLines(string(data))
	if len(lines) < 3 {
		return spec.Model{}, fmt.Errorf("%w: front matter is incomplete", ErrMalformedMarkdown)
	}
	end := -1
	inFence := false
	phaseIDs := map[string]bool{}
	for i := 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "```") {
			inFence = !inFence
		}
		if !inFence && lines[i] == "---" {
			end = i
			break
		}
	}
	if inFence {
		return spec.Model{}, fmt.Errorf("%w: unclosed code fence", ErrMalformedMarkdown)
	}
	if end < 0 {
		return spec.Model{}, fmt.Errorf("%w: closing front matter fence is missing", ErrMalformedMarkdown)
	}
	front := parseFrontMatter(lines[1:end])
	model := spec.Model{
		Version:      front["spec_version"],
		TaskID:       front["task_id"],
		Created:      front["created"],
		Updated:      front["updated"],
		Status:       spec.Status(front["status"]),
		HardenStatus: spec.HardenStatus(front["harden_status"]),
		Size:         spec.Size(front["size"]),
		RiskLevel:    spec.RiskLevel(front["risk_level"]),
		Metadata:     map[string]string{},
	}
	body := lines[end+1:]
	var section string
	var phase *spec.Phase
	var criterion *spec.Criterion
	var phaseField string
	var listTarget *[]string
	inFence = false
	for _, line := range body {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			model.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			continue
		}
		if strings.HasPrefix(line, "## ") {
			if match := phaseHeadingPattern.FindStringSubmatch(line); match != nil {
				number, _ := strconv.Atoi(match[1])
				id := fmt.Sprintf("phase%d", number)
				if phaseIDs[id] {
					return spec.Model{}, fmt.Errorf("%w: duplicate phase heading %s", ErrMalformedMarkdown, id)
				}
				phaseIDs[id] = true
				model.Phases = append(model.Phases, spec.Phase{ID: id, Number: number, Name: match[2], Status: "pending"})
				phase = &model.Phases[len(model.Phases)-1]
				section = "phase"
				criterion = nil
				phaseField = ""
				continue
			}
			section = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			phase = nil
			criterion = nil
			phaseField = ""
			listTarget = listForSection(&model, section)
			continue
		}
		if section == "summary" && strings.TrimSpace(line) != "" {
			if model.Summary != "" {
				model.Summary += "\n"
			}
			model.Summary += line
			continue
		}
		if section == "current state" {
			parseCurrentStateLine(&model, line)
			continue
		}
		if section == "acceptance" {
			if strings.HasPrefix(line, "Profile:") {
				model.Acceptance.ValidationProfile = strings.TrimSpace(strings.TrimPrefix(line, "Profile:"))
				continue
			}
			if line == "Validation:" {
				continue
			}
		}
		if section == "review" {
			if strings.HasPrefix(line, "Status:") {
				model.Review.Status = strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
				continue
			}
			if strings.HasPrefix(line, "Verdict:") {
				model.Review.Verdict = strings.TrimSpace(strings.TrimPrefix(line, "Verdict:"))
				continue
			}
		}
		if section == "metadata" && strings.HasPrefix(line, "- ") {
			key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), ":")
			if ok && strings.TrimSpace(key) != "none" {
				model.Metadata[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
			continue
		}
		if section == "origin" {
			if strings.HasPrefix(line, "Created by:") {
				model.Origin.CreatedBy = strings.TrimSpace(strings.TrimPrefix(line, "Created by:"))
			}
			if strings.HasPrefix(line, "Source:") {
				model.Origin.Source = strings.TrimSpace(strings.TrimPrefix(line, "Source:"))
			}
			continue
		}
		if listTarget != nil && strings.HasPrefix(line, "- ") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			if value != "none" && value != "" {
				*listTarget = append(*listTarget, value)
			}
			continue
		}
		if strings.HasPrefix(line, "Status:") {
			status := strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
			if phase != nil {
				phase.Status = status
			}
			continue
		}
		if phase != nil {
			if strings.HasPrefix(line, "Objective:") {
				phase.Objective = strings.TrimSpace(strings.TrimPrefix(line, "Objective:"))
				continue
			}
			if strings.HasPrefix(line, "Dependencies:") {
				phase.Dependencies = splitCSV(strings.TrimSpace(strings.TrimPrefix(line, "Dependencies:")))
				continue
			}
			if line == "Changes:" {
				phaseField = "changes"
				continue
			}
			if line == "Acceptance:" {
				phaseField = "acceptance"
				continue
			}
			if phaseField == "changes" && strings.HasPrefix(line, "- ") {
				value := strings.TrimSpace(strings.TrimPrefix(line, "- "))
				if value != "none" {
					phase.Changes = append(phase.Changes, value)
				}
				continue
			}
		}
		if match := criterionLinePattern.FindStringSubmatch(line); match != nil {
			c := spec.Criterion{ID: match[1], Type: strings.TrimSpace(match[2]), Title: strings.TrimSpace(match[3]), ExpectedKind: acceptance.ExpectedExitCodeZero, Status: "pending"}
			if phase != nil {
				c.PhaseID = phase.ID
				phase.Acceptance = append(phase.Acceptance, c)
				criterion = &phase.Acceptance[len(phase.Acceptance)-1]
			} else {
				model.Acceptance.Criteria = append(model.Acceptance.Criteria, c)
				criterion = &model.Acceptance.Criteria[len(model.Acceptance.Criteria)-1]
			}
			continue
		}
		if criterion != nil {
			if match := commandLinePattern.FindStringSubmatch(line); match != nil {
				criterion.Command = match[1]
			}
			if match := expectedLinePattern.FindStringSubmatch(line); match != nil {
				criterion.ExpectedKind = acceptance.ExpectedKind(match[1])
			}
			if match := statusLinePattern.FindStringSubmatch(line); match != nil {
				criterion.Status = match[1]
			}
			if match := evidenceLinePattern.FindStringSubmatch(line); match != nil {
				criterion.Evidence = match[1]
			}
			if match := sourceLinePattern.FindStringSubmatch(line); match != nil {
				criterion.SourceEvent = match[1]
			}
		}
	}
	if inFence {
		return spec.Model{}, fmt.Errorf("%w: unclosed code fence", ErrMalformedMarkdown)
	}
	return model, nil
}

func Render(model spec.Model) []byte {
	var b strings.Builder
	writeFrontMatter(&b, model)
	fmt.Fprintf(&b, "# %s\n\n", fallback(model.Title, model.TaskID))
	fmt.Fprintf(&b, "## Current State\n\n")
	fmt.Fprintf(&b, "Status: %s\n", fallback(string(model.Status), "draft"))
	fmt.Fprintf(&b, "Current phase: %s\n", fallback(model.CurrentState.CurrentPhase, "none"))
	fmt.Fprintf(&b, "Next: %s\n", fallback(model.CurrentState.Next, "approve"))
	fmt.Fprintf(&b, "Reason: %s\n", fallback(model.CurrentState.Reason, "new task spec"))
	fmt.Fprintf(&b, "Blockers: %s\n", fallback(model.CurrentState.Blockers, "none"))
	fmt.Fprintf(&b, "Allowed follow-up command: `%s`\n", fallback(model.CurrentState.AllowedFollowUp, "scafld status "+model.TaskID))
	fmt.Fprintf(&b, "Latest runner update: %s\n", fallback(model.CurrentState.LatestRunnerUpdate, "none"))
	fmt.Fprintf(&b, "Review gate: %s\n\n", fallback(model.CurrentState.ReviewGate, "not_started"))
	fmt.Fprintf(&b, "## Summary\n\n%s\n\n", fallback(model.Summary, "No summary yet."))
	renderContext(&b, model.Context)
	renderStringList(&b, "Objectives", model.Objectives)
	renderStringList(&b, "Scope", model.Scope)
	renderStringList(&b, "Dependencies", model.Dependencies)
	renderStringList(&b, "Assumptions", model.Assumptions)
	renderStringList(&b, "Touchpoints", model.Touchpoints)
	renderRisks(&b, model.Risks)
	fmt.Fprintf(&b, "## Acceptance\n\nProfile: %s\n\nValidation:\n", fallback(model.Acceptance.ValidationProfile, "standard"))
	renderCriteria(&b, model.Acceptance.Criteria)
	if len(model.Acceptance.Criteria) == 0 {
		fmt.Fprintf(&b, "- none\n")
	}
	fmt.Fprintf(&b, "\n")
	for _, phase := range model.Phases {
		number := phase.Number
		if number == 0 {
			number = len(model.Phases)
		}
		fmt.Fprintf(&b, "## Phase %d: %s\n\n", number, fallback(phase.Name, phase.ID))
		fmt.Fprintf(&b, "Status: %s\n", fallback(phase.Status, "pending"))
		fmt.Fprintf(&b, "Dependencies: %s\n\n", fallback(strings.Join(phase.Dependencies, ", "), "none"))
		fmt.Fprintf(&b, "Objective: %s\n\n", fallback(phase.Objective, "Complete this phase."))
		fmt.Fprintf(&b, "Changes:\n")
		renderBullets(&b, phase.Changes)
		fmt.Fprintf(&b, "\nAcceptance:\n")
		renderCriteria(&b, phase.Acceptance)
		if len(phase.Acceptance) == 0 {
			fmt.Fprintf(&b, "- none\n")
		}
		fmt.Fprintf(&b, "\n")
	}
	renderStringList(&b, "Rollback", model.Rollback)
	fmt.Fprintf(&b, "## Review\n\nStatus: %s\nVerdict: %s\n\n", fallback(model.Review.Status, "not_started"), fallback(model.Review.Verdict, "none"))
	renderStringList(&b, "Self Eval", model.SelfEval)
	renderStringList(&b, "Deviations", model.Deviations)
	fmt.Fprintf(&b, "## Metadata\n\n")
	if len(model.Metadata) == 0 {
		fmt.Fprintf(&b, "- none\n")
	} else {
		keys := make([]string, 0, len(model.Metadata))
		for key := range model.Metadata {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "- %s: %s\n", key, model.Metadata[key])
		}
	}
	fmt.Fprintf(&b, "\n## Origin\n\nCreated by: %s\nSource: %s\n\n", fallback(model.Origin.CreatedBy, "scafld"), fallback(model.Origin.Source, "plan"))
	fmt.Fprintf(&b, "## Harden Rounds\n\n")
	if len(model.HardenRounds) == 0 {
		fmt.Fprintf(&b, "- none\n")
	} else {
		for _, round := range model.HardenRounds {
			fmt.Fprintf(&b, "- %s: %s\n", round.ID, round.Status)
		}
	}
	fmt.Fprintf(&b, "\n## Planning Log\n\n")
	if len(model.PlanningLog) == 0 {
		fmt.Fprintf(&b, "- none\n")
	} else {
		for _, event := range model.PlanningLog {
			fmt.Fprintf(&b, "- %s %s\n", event.Time, event.Text)
		}
	}
	return []byte(b.String())
}

func writeSpec(ctx context.Context, path string, model spec.Model) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create spec dir: %w", err)
	}
	data := Render(model)
	if err := atomicfile.Write(path, data, 0o644); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}
	return nil
}

func parseFrontMatter(lines []string) map[string]string {
	values := map[string]string{}
	for _, line := range lines {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `'"`)
	}
	return values
}

func writeFrontMatter(b *strings.Builder, model spec.Model) {
	fmt.Fprintf(b, "---\n")
	fmt.Fprintf(b, "spec_version: '%s'\n", fallback(model.Version, "2.0"))
	fmt.Fprintf(b, "task_id: %s\n", model.TaskID)
	fmt.Fprintf(b, "created: '%s'\n", model.Created)
	fmt.Fprintf(b, "updated: '%s'\n", model.Updated)
	fmt.Fprintf(b, "status: %s\n", fallback(string(model.Status), "draft"))
	fmt.Fprintf(b, "harden_status: %s\n", fallback(string(model.HardenStatus), "not_run"))
	fmt.Fprintf(b, "size: %s\n", fallback(string(model.Size), "medium"))
	fmt.Fprintf(b, "risk_level: %s\n", fallback(string(model.RiskLevel), "medium"))
	fmt.Fprintf(b, "---\n\n")
}

func renderStringList(b *strings.Builder, title string, items []string) {
	fmt.Fprintf(b, "## %s\n\n", title)
	renderBullets(b, items)
	fmt.Fprintf(b, "\n")
}

func renderContext(b *strings.Builder, context spec.Context) {
	if context.CWD == "" && len(context.Packages) == 0 && len(context.FilesImpacted) == 0 && len(context.Invariants) == 0 && len(context.RelatedDocs) == 0 {
		return
	}
	fmt.Fprintf(b, "## Context\n\n")
	if context.CWD != "" {
		fmt.Fprintf(b, "CWD: `%s`\n\n", context.CWD)
	}
	renderBullets(b, context.Packages)
	fmt.Fprintf(b, "\n")
}

func renderRisks(b *strings.Builder, risks []spec.Risk) {
	fmt.Fprintf(b, "## Risks\n\n")
	if len(risks) == 0 {
		fmt.Fprintf(b, "- none\n\n")
		return
	}
	for _, risk := range risks {
		fmt.Fprintf(b, "- %s", risk.Description)
		if risk.Mitigation != "" {
			fmt.Fprintf(b, " - %s", risk.Mitigation)
		}
		fmt.Fprintf(b, "\n")
	}
	fmt.Fprintf(b, "\n")
}

func renderBullets(b *strings.Builder, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(b, "- none\n")
		return
	}
	for _, item := range items {
		fmt.Fprintf(b, "- %s\n", item)
	}
}

func renderCriteria(b *strings.Builder, criteria []spec.Criterion) {
	for _, c := range criteria {
		fmt.Fprintf(b, "- [%s] `%s` %s - %s\n", checked(c.Status), c.ID, fallback(c.Type, "command"), fallback(c.Title, c.Command))
		if c.Command != "" {
			fmt.Fprintf(b, "  - Command: `%s`\n", c.Command)
		}
		fmt.Fprintf(b, "  - Expected kind: `%s`\n", fallback(string(c.ExpectedKind), string(acceptance.ExpectedExitCodeZero)))
		fmt.Fprintf(b, "  - Status: %s\n", fallback(c.Status, "pending"))
		if c.Evidence != "" {
			fmt.Fprintf(b, "  - Evidence: %s\n", c.Evidence)
		}
		if c.SourceEvent != "" {
			fmt.Fprintf(b, "  - Source event: %s\n", c.SourceEvent)
		}
	}
}

func checked(status string) string {
	if status == "pass" || status == "completed" {
		return "x"
	}
	return " "
}

func fallback(value, fb string) string {
	if strings.TrimSpace(value) == "" {
		return fb
	}
	return value
}

func splitLines(text string) []string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func listForSection(model *spec.Model, section string) *[]string {
	switch section {
	case "objectives":
		return &model.Objectives
	case "scope":
		return &model.Scope
	case "dependencies":
		return &model.Dependencies
	case "assumptions":
		return &model.Assumptions
	case "touchpoints":
		return &model.Touchpoints
	case "rollback":
		return &model.Rollback
	case "self eval":
		return &model.SelfEval
	case "deviations":
		return &model.Deviations
	default:
		return nil
	}
}

func parseCurrentStateLine(model *spec.Model, line string) {
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		return
	}
	value = strings.Trim(strings.TrimSpace(value), "`")
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "current phase":
		model.CurrentState.CurrentPhase = value
	case "next":
		model.CurrentState.Next = value
	case "reason":
		model.CurrentState.Reason = value
	case "blockers":
		model.CurrentState.Blockers = value
	case "allowed follow-up command":
		model.CurrentState.AllowedFollowUp = value
	case "latest runner update":
		model.CurrentState.LatestRunnerUpdate = value
	case "review gate":
		model.CurrentState.ReviewGate = value
	}
}

func splitCSV(value string) []string {
	if value == "" || value == "none" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
