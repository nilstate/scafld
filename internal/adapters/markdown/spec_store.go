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
	"time"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
)

var (
	// ErrSpecNotFound is returned when taskID cannot be found in spec directories.
	ErrSpecNotFound = errors.New("spec not found")
	// ErrSpecExists is returned when a write would overwrite another task spec.
	ErrSpecExists = errors.New("spec already exists")
	// ErrMalformedMarkdown wraps spec Markdown grammar failures.
	ErrMalformedMarkdown = errors.New("malformed markdown spec")
	phaseHeadingPattern  = regexp.MustCompile(`^## Phase ([0-9]+):\s*(.+?)\s*$`)
	criterionLinePattern = regexp.MustCompile(`^- \[[ xX]\] ` + "`" + `([^` + "`" + `]+)` + "`" + `\s+([^-]+?)\s+-\s*(.*)$`)
	findingLinePattern   = regexp.MustCompile(`^- \[([a-z_]+)\]\s+` + "`" + `([^` + "`" + `]+)` + "`" + `\s+(.*)$`)
)

// Store reads and writes Markdown task specs under a workspace root.
type Store struct {
	Root string
}

// CreateDraft writes model as a new draft spec and returns its path.
func (s Store) CreateDraft(ctx context.Context, model spec.Model) (string, error) {
	root := s.Root
	if root == "" {
		root = "."
	}
	path := filepath.Join(root, ".scafld", "specs", "drafts", model.TaskID+".md")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("%w: %s", ErrSpecExists, path)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat draft spec: %w", err)
	}
	return path, writeSpec(ctx, path, model)
}

// Save writes model to an existing spec path.
func (s Store) Save(ctx context.Context, path string, model spec.Model) error {
	root := s.Root
	if root == "" {
		root = "."
	}
	return updateSpec(ctx, root, path, model)
}

// Load finds, parses, and returns a spec by task ID. If the file's directory
// disagrees with its frontmatter status, Load relocates it to the status-implied
// directory before returning. Relocation failures are non-fatal.
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
	root := s.Root
	if root == "" {
		root = "."
	}
	if target := targetPath(root, model); !samePath(path, target) {
		if err := writeMovedSpec(path, target, data); err == nil {
			path = target
		}
	}
	return model, path, nil
}

// Find returns the path for taskID across active spec directories.
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

// List returns summaries for current non-archived task specs.
func (s Store) List(ctx context.Context) ([]spec.Record, error) {
	return s.list(ctx, false)
}

// ListAll returns summaries for current and archived task specs.
func (s Store) ListAll(ctx context.Context) ([]spec.Record, error) {
	return s.list(ctx, true)
}

func (s Store) list(ctx context.Context, includeArchive bool) ([]spec.Record, error) {
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
	if includeArchive {
		base := filepath.Join(root, ".scafld", "specs", "archive")
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			model, err := Parse(data)
			if err != nil {
				return err
			}
			records = append(records, spec.Record{TaskID: model.TaskID, Status: model.Status, Path: path, Title: model.Title})
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].TaskID < records[j].TaskID })
	return records, nil
}

// Parse converts living Markdown spec bytes into a normalized model.
func Parse(data []byte) (spec.Model, error) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return spec.Model{}, fmt.Errorf("%w: front matter is required", ErrMalformedMarkdown)
	}
	lines := splitLines(string(data))
	if len(lines) < 3 {
		return spec.Model{}, fmt.Errorf("%w: front matter is incomplete", ErrMalformedMarkdown)
	}
	end, err := frontMatterEnd(lines)
	if err != nil {
		return spec.Model{}, err
	}
	front := parseFrontMatter(lines[1:end])
	parser := newParser(front)
	if err := parser.parseBody(lines[end+1:]); err != nil {
		return spec.Model{}, err
	}
	return parser.model, nil
}

func frontMatterEnd(lines []string) (int, error) {
	inFence := false
	for i := 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "```") {
			inFence = !inFence
		}
		if !inFence && lines[i] == "---" {
			return i, nil
		}
	}
	if inFence {
		return 0, fmt.Errorf("%w: unclosed code fence", ErrMalformedMarkdown)
	}
	return 0, fmt.Errorf("%w: closing front matter fence is missing", ErrMalformedMarkdown)
}

type parser struct {
	model          spec.Model
	section        string
	phase          *spec.Phase
	criterion      *spec.Criterion
	hardenRound    *spec.HardenRound
	hardenQuestion *spec.HardenQuestion
	phaseField     string
	hardenField    string
	listTarget     *([]string)
	phaseIDs       map[string]bool
}

func newParser(front map[string]string) *parser {
	return &parser{
		model: spec.Model{
			Version:      front["spec_version"],
			TaskID:       front["task_id"],
			Created:      front["created"],
			Updated:      front["updated"],
			Status:       spec.Status(front["status"]),
			HardenStatus: spec.HardenStatus(front["harden_status"]),
			Size:         spec.Size(front["size"]),
			RiskLevel:    spec.RiskLevel(front["risk_level"]),
			Metadata:     map[string]string{},
		},
		phaseIDs: map[string]bool{},
	}
}

func (p *parser) parseBody(lines []string) error {
	inFence := false
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
		}
		if inFence {
			continue
		}
		handled, err := p.handleHeading(line)
		if err != nil {
			return err
		}
		if handled {
			continue
		}
		p.handleLine(line)
	}
	if inFence {
		return fmt.Errorf("%w: unclosed code fence", ErrMalformedMarkdown)
	}
	return nil
}

func (p *parser) handleHeading(line string) (bool, error) {
	if strings.HasPrefix(line, "# ") {
		p.model.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
		return true, nil
	}
	if !strings.HasPrefix(line, "## ") {
		return false, nil
	}
	if match := phaseHeadingPattern.FindStringSubmatch(line); match != nil {
		return true, p.startPhase(match)
	}
	p.startSection(strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## "))))
	return true, nil
}

func (p *parser) startPhase(match []string) error {
	number, _ := strconv.Atoi(match[1])
	id := fmt.Sprintf("phase%d", number)
	if p.phaseIDs[id] {
		return fmt.Errorf("%w: duplicate phase heading %s", ErrMalformedMarkdown, id)
	}
	p.phaseIDs[id] = true
	p.model.Phases = append(p.model.Phases, spec.Phase{ID: id, Number: number, Name: match[2], Status: "pending"})
	p.phase = &p.model.Phases[len(p.model.Phases)-1]
	p.section = "phase"
	p.criterion = nil
	p.hardenRound = nil
	p.hardenQuestion = nil
	p.phaseField = ""
	p.hardenField = ""
	p.listTarget = nil
	return nil
}

func (p *parser) startSection(section string) {
	p.section = section
	p.phase = nil
	p.criterion = nil
	p.hardenRound = nil
	p.hardenQuestion = nil
	p.phaseField = ""
	p.hardenField = ""
	p.listTarget = listForSection(&p.model, section)
}

func (p *parser) handleLine(line string) {
	switch {
	case p.appendSummary(line):
	case p.handleCurrentState(line):
	case p.handleAcceptance(line):
	case p.handleReview(line):
	case p.handleMetadata(line):
	case p.handleOrigin(line):
	case p.handleHardenLine(line):
	case p.handleListItem(line):
	case p.handlePhaseLine(line):
	case p.handleCriterionStart(line):
	default:
		p.handleCriterionDetail(line)
	}
}

func (p *parser) appendSummary(line string) bool {
	if p.section != "summary" || strings.TrimSpace(line) == "" {
		return false
	}
	if p.model.Summary != "" {
		p.model.Summary += "\n"
	}
	p.model.Summary += line
	return true
}

func (p *parser) handleCurrentState(line string) bool {
	if p.section != "current state" {
		return false
	}
	parseCurrentStateLine(&p.model, line)
	return true
}

func (p *parser) handleAcceptance(line string) bool {
	if p.section != "acceptance" {
		return false
	}
	key, value, ok := parseSpecKeyValue(line)
	if !ok {
		return false
	}
	switch normalizeSpecKey(key) {
	case "profile":
		p.model.Acceptance.ValidationProfile = value
	case "validation":
	default:
		return false
	}
	return true
}

func (p *parser) handleReview(line string) bool {
	if p.section != "review" {
		return false
	}
	if key, value, ok := parseSpecKeyValue(line); ok {
		switch normalizeSpecKey(key) {
		case "status":
			p.model.Review.Status = value
			return true
		case "verdict":
			p.model.Review.Verdict = value
			return true
		case "findings":
			return true
		}
	}
	if match := findingLinePattern.FindStringSubmatch(line); match != nil {
		p.model.Review.Findings = append(p.model.Review.Findings, reviewFinding(match[2], match[1], match[3]))
		return true
	}
	return false
}

func (p *parser) handleMetadata(line string) bool {
	if p.section != "metadata" || !strings.HasPrefix(line, "- ") {
		return false
	}
	key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), ":")
	if ok && strings.TrimSpace(key) != "none" {
		p.model.Metadata[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return true
}

func (p *parser) handleOrigin(line string) bool {
	if p.section != "origin" {
		return false
	}
	key, value, ok := parseSpecKeyValue(line)
	if !ok {
		return false
	}
	switch normalizeSpecKey(key) {
	case "created by":
		p.model.Origin.CreatedBy = value
	case "source":
		p.model.Origin.Source = value
	default:
		return false
	}
	return true
}

func (p *parser) handleHardenLine(line string) bool {
	if p.section != "harden rounds" {
		return false
	}
	trimmed := strings.TrimSpace(line)
	switch {
	case trimmed == "":
		return true
	case strings.HasPrefix(line, "### "):
		p.startHardenRound(strings.TrimSpace(strings.TrimPrefix(line, "### ")))
		return true
	}
	if key, value, ok := parseSpecKeyValue(line); ok {
		switch normalizeSpecKey(key) {
		case "status":
			if p.hardenRound != nil {
				p.hardenRound.Status = value
			}
			return true
		case "started":
			if p.hardenRound != nil {
				p.hardenRound.StartedAt = noneToEmpty(value)
			}
			return true
		case "ended":
			if p.hardenRound != nil {
				p.hardenRound.EndedAt = noneToEmpty(value)
			}
			return true
		case "questions":
			p.hardenField = "questions"
			return true
		}
	}
	if strings.HasPrefix(line, "- ") {
		return p.handleHardenBullet(strings.TrimSpace(strings.TrimPrefix(line, "- ")))
	}
	if strings.HasPrefix(line, "  - ") {
		return p.handleHardenQuestionDetail(strings.TrimSpace(strings.TrimPrefix(line, "  - ")))
	}
	if strings.HasPrefix(line, "  ") {
		return p.handleHardenQuestionDetail(strings.TrimSpace(line))
	}
	return true
}

func (p *parser) startHardenRound(id string) {
	p.model.HardenRounds = append(p.model.HardenRounds, spec.HardenRound{ID: id})
	p.hardenRound = &p.model.HardenRounds[len(p.model.HardenRounds)-1]
	p.hardenQuestion = nil
	p.hardenField = ""
}

func (p *parser) handleHardenBullet(value string) bool {
	if value == "" || value == "none" {
		return true
	}
	if p.hardenRound == nil {
		id, status, ok := strings.Cut(value, ":")
		if ok {
			p.model.HardenRounds = append(p.model.HardenRounds, spec.HardenRound{
				ID:     strings.TrimSpace(id),
				Status: strings.TrimSpace(status),
			})
		}
		return true
	}
	if p.hardenField == "questions" {
		if key, body, ok := parseSpecKeyValue(value); ok && normalizeSpecKey(key) == "question" {
			p.startHardenQuestion(body)
			return true
		}
		p.startHardenQuestion(value)
	}
	return true
}

func (p *parser) startHardenQuestion(value string) {
	if p.hardenRound == nil {
		return
	}
	value = cleanSpecValue(value)
	if value == "" {
		return
	}
	normalized := normalizeHardenQuestion(value)
	for i := range p.hardenRound.Questions {
		if normalizeHardenQuestion(p.hardenRound.Questions[i].Question) == normalized {
			p.hardenQuestion = &p.hardenRound.Questions[i]
			return
		}
	}
	p.hardenRound.Questions = append(p.hardenRound.Questions, spec.HardenQuestion{Question: value})
	p.hardenQuestion = &p.hardenRound.Questions[len(p.hardenRound.Questions)-1]
}

func (p *parser) handleHardenQuestionDetail(value string) bool {
	key, body, ok := parseSpecKeyValue(value)
	if !ok {
		return true
	}
	if normalizeSpecKey(key) == "question" {
		p.startHardenQuestion(body)
		return true
	}
	if p.hardenQuestion == nil {
		return true
	}
	switch normalizeSpecKey(key) {
	case "grounded in":
		p.hardenQuestion.GroundedIn = body
	case "recommended answer":
		p.hardenQuestion.RecommendedAnswer = body
	case "if unanswered":
		p.hardenQuestion.IfUnanswered = body
	case "answered with", "resolution":
		p.hardenQuestion.AnsweredWith = body
	}
	return true
}

func normalizeHardenQuestion(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func (p *parser) handleListItem(line string) bool {
	if p.listTarget == nil || !strings.HasPrefix(line, "- ") {
		return false
	}
	value := strings.TrimSpace(strings.TrimPrefix(line, "- "))
	if value != "none" && value != "" {
		*p.listTarget = append(*p.listTarget, value)
	}
	return true
}

func (p *parser) handlePhaseLine(line string) bool {
	if p.phase == nil {
		return false
	}
	if p.phaseField == "changes" && strings.HasPrefix(line, "- ") {
		p.appendPhaseChange(line)
		return true
	}
	if strings.HasPrefix(strings.TrimSpace(line), "- ") {
		return false
	}
	key, value, ok := parseSpecKeyValue(line)
	if !ok {
		return false
	}
	switch normalizeSpecKey(key) {
	case "status":
		p.phase.Status = value
	case "objective":
		p.phase.Objective = value
	case "dependencies":
		p.phase.Dependencies = splitCSV(value)
	case "changes":
		p.phaseField = "changes"
	case "acceptance":
		p.phaseField = "acceptance"
	default:
		return false
	}
	return true
}

func (p *parser) appendPhaseChange(line string) {
	value := strings.TrimSpace(strings.TrimPrefix(line, "- "))
	if value != "none" {
		p.phase.Changes = append(p.phase.Changes, value)
	}
}

func (p *parser) handleCriterionStart(line string) bool {
	match := criterionLinePattern.FindStringSubmatch(line)
	if match == nil {
		return false
	}
	c := spec.Criterion{ID: match[1], Type: strings.TrimSpace(match[2]), Title: strings.TrimSpace(match[3]), ExpectedKind: acceptance.ExpectedExitCodeZero, Status: "pending"}
	if p.phase != nil {
		c.PhaseID = p.phase.ID
		p.phase.Acceptance = append(p.phase.Acceptance, c)
		p.criterion = &p.phase.Acceptance[len(p.phase.Acceptance)-1]
		return true
	}
	p.model.Acceptance.Criteria = append(p.model.Acceptance.Criteria, c)
	p.criterion = &p.model.Acceptance.Criteria[len(p.model.Acceptance.Criteria)-1]
	return true
}

func (p *parser) handleCriterionDetail(line string) {
	if p.criterion == nil {
		return
	}
	key, value, ok := parseSpecListKeyValue(line)
	if !ok {
		return
	}
	switch normalizeSpecKey(key) {
	case "command":
		p.criterion.Command = value
	case "expected kind":
		p.criterion.ExpectedKind = acceptance.ExpectedKind(value)
	case "status":
		p.criterion.Status = value
	case "evidence":
		p.criterion.Evidence = value
	case "source event":
		p.criterion.SourceEvent = value
	}
}

// Render converts a normalized model into canonical living Markdown.
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
	renderReview(&b, model.Review)
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
			renderHardenRound(&b, round)
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

func renderHardenRound(b *strings.Builder, round spec.HardenRound) {
	fmt.Fprintf(b, "### %s\n\n", fallback(round.ID, "round"))
	fmt.Fprintf(b, "Status: %s\n", fallback(round.Status, "in_progress"))
	fmt.Fprintf(b, "Started: %s\n", fallback(round.StartedAt, "none"))
	fmt.Fprintf(b, "Ended: %s\n\n", fallback(round.EndedAt, "none"))
	fmt.Fprintf(b, "Questions:\n")
	if len(round.Questions) == 0 {
		fmt.Fprintf(b, "- none\n\n")
		return
	}
	for _, question := range round.Questions {
		fmt.Fprintf(b, "- %s\n", fallback(question.Question, "Question not recorded."))
		if question.GroundedIn != "" {
			fmt.Fprintf(b, "  - Grounded in: %s\n", question.GroundedIn)
		}
		if question.RecommendedAnswer != "" {
			fmt.Fprintf(b, "  - Recommended answer: %s\n", question.RecommendedAnswer)
		}
		if question.IfUnanswered != "" {
			fmt.Fprintf(b, "  - If unanswered: %s\n", question.IfUnanswered)
		}
		if question.AnsweredWith != "" {
			fmt.Fprintf(b, "  - Answered with: %s\n", question.AnsweredWith)
		}
	}
	fmt.Fprintf(b, "\n")
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

func updateSpec(ctx context.Context, root string, path string, model spec.Model) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read existing spec: %w", err)
	}
	previous, err := Parse(current)
	if err != nil {
		return err
	}
	if !samePhaseShape(previous.Phases, model.Phases) {
		return fmt.Errorf("%w: phase shape changed during targeted save", ErrMalformedMarkdown)
	}
	data, err := updateSpecMarkdown(current, previous, model)
	if err != nil {
		return err
	}
	target := targetPath(root, model)
	if samePath(path, target) {
		if err := atomicfile.Write(path, data, 0o644); err != nil {
			return fmt.Errorf("write spec: %w", err)
		}
		return nil
	}
	if err := writeMovedSpec(path, target, data); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}
	return nil
}

func targetPath(root string, model spec.Model) string {
	dir := "active"
	switch model.Status {
	case spec.StatusDraft:
		dir = "drafts"
	case spec.StatusApproved:
		dir = "approved"
	case spec.StatusCompleted, spec.StatusFailed, spec.StatusCancelled:
		return filepath.Join(root, ".scafld", "specs", "archive", archiveMonth(model.Updated), model.TaskID+".md")
	}
	return filepath.Join(root, ".scafld", "specs", dir, model.TaskID+".md")
}

func archiveMonth(updated string) string {
	if ts, err := time.Parse(time.RFC3339, updated); err == nil {
		return ts.UTC().Format("2006-01")
	}
	if len(updated) >= len("2006-01") {
		return updated[:len("2006-01")]
	}
	return time.Now().UTC().Format("2006-01")
}

func writeMovedSpec(source string, target string, data []byte) error {
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%w: %s", ErrSpecExists, target)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat target: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}
	if err := atomicfile.Write(target, data, 0o644); err != nil {
		return err
	}
	if err := os.Remove(source); err != nil {
		_ = os.Remove(target)
		return fmt.Errorf("remove old spec: %w", err)
	}
	return nil
}

func samePath(a string, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil {
		return filepath.Clean(absA) == filepath.Clean(absB)
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// updateSpecMarkdown applies only the sections whose normalized model changed.
// Parser-owned fields can move forward without re-rendering human-owned prose
// that the normalized model intentionally does not understand.
func updateSpecMarkdown(current []byte, previous spec.Model, next spec.Model) ([]byte, error) {
	currentDoc, err := splitDocument(string(current))
	if err != nil {
		return nil, err
	}
	previousDoc, err := splitDocument(string(Render(previous)))
	if err != nil {
		return nil, err
	}
	nextDoc, err := splitDocument(string(Render(next)))
	if err != nil {
		return nil, err
	}
	replacements := map[string]string{}
	for key, nextSegment := range nextDoc.segments {
		if previousDoc.segments[key] != nextSegment {
			replacements[key] = nextSegment
		}
	}
	if len(replacements) == 0 {
		return current, nil
	}
	var b strings.Builder
	seen := map[string]bool{}
	for _, key := range currentDoc.order {
		seen[key] = true
		if replacement, ok := replacements[key]; ok {
			b.WriteString(replacement)
			continue
		}
		b.WriteString(currentDoc.segments[key])
	}
	for _, key := range nextDoc.order {
		if seen[key] {
			continue
		}
		replacement, ok := replacements[key]
		if !ok {
			continue
		}
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteString("\n")
		}
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n\n") {
			b.WriteString("\n")
		}
		b.WriteString(replacement)
	}
	return []byte(b.String()), nil
}

type documentSections struct {
	order    []string
	segments map[string]string
}

type lineSpan struct {
	start int
	end   int
	text  string
}

func splitDocument(text string) (documentSections, error) {
	lines := spanLines(text)
	if len(lines) < 3 || lines[0].text != "---" {
		return documentSections{}, fmt.Errorf("%w: front matter is required", ErrMalformedMarkdown)
	}
	end, err := frontMatterEnd(lineTexts(lines))
	if err != nil {
		return documentSections{}, err
	}
	doc := documentSections{segments: map[string]string{}}
	addSegment := func(key string, start int, end int) error {
		if start >= end {
			return nil
		}
		if _, exists := doc.segments[key]; exists {
			if isManagedSectionKey(key) {
				return fmt.Errorf("%w: duplicate section %s", ErrMalformedMarkdown, key)
			}
			for index := 2; ; index++ {
				candidate := fmt.Sprintf("%s#%d", key, index)
				if _, exists := doc.segments[candidate]; !exists {
					key = candidate
					break
				}
			}
		}
		doc.order = append(doc.order, key)
		doc.segments[key] = text[start:end]
		return nil
	}
	if err := addSegment("__front_matter__", 0, lines[end].end); err != nil {
		return documentSections{}, err
	}
	bodyStart := len(text)
	if end+1 < len(lines) {
		bodyStart = lines[end+1].start
	}
	var starts []struct {
		key   string
		start int
	}
	inFence := false
	for i := end + 1; i < len(lines); i++ {
		line := lines[i].text
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			starts = append(starts, struct {
				key   string
				start int
			}{key: "__title__", start: lines[i].start})
			continue
		}
		if strings.HasPrefix(line, "## ") {
			key := sectionKey(line)
			starts = append(starts, struct {
				key   string
				start int
			}{key: key, start: lines[i].start})
		}
	}
	if inFence {
		return documentSections{}, fmt.Errorf("%w: unclosed code fence", ErrMalformedMarkdown)
	}
	if len(starts) == 0 {
		if bodyStart < len(text) {
			if err := addSegment("__body__", bodyStart, len(text)); err != nil {
				return documentSections{}, err
			}
		}
		return doc, nil
	}
	if bodyStart < starts[0].start {
		if err := addSegment("__preamble__", bodyStart, starts[0].start); err != nil {
			return documentSections{}, err
		}
	}
	for i, start := range starts {
		endOffset := len(text)
		if i+1 < len(starts) {
			endOffset = starts[i+1].start
		}
		if err := addSegment(start.key, start.start, endOffset); err != nil {
			return documentSections{}, err
		}
	}
	return doc, nil
}

func sectionKey(line string) string {
	if match := phaseHeadingPattern.FindStringSubmatch(line); match != nil {
		return "phase:" + fmt.Sprintf("phase%s", match[1])
	}
	return "section:" + strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
}

func isManagedSectionKey(key string) bool {
	if key == "__front_matter__" || key == "__title__" || strings.HasPrefix(key, "phase:") {
		return true
	}
	switch key {
	case "section:current state",
		"section:summary",
		"section:context",
		"section:objectives",
		"section:scope",
		"section:dependencies",
		"section:assumptions",
		"section:touchpoints",
		"section:risks",
		"section:acceptance",
		"section:rollback",
		"section:review",
		"section:self eval",
		"section:deviations",
		"section:metadata",
		"section:origin",
		"section:harden rounds",
		"section:planning log":
		return true
	default:
		return false
	}
}

func spanLines(text string) []lineSpan {
	if text == "" {
		return nil
	}
	var lines []lineSpan
	start := 0
	for start < len(text) {
		next := strings.IndexByte(text[start:], '\n')
		if next == -1 {
			lines = append(lines, lineSpan{start: start, end: len(text), text: text[start:]})
			break
		}
		end := start + next + 1
		lines = append(lines, lineSpan{start: start, end: end, text: text[start : end-1]})
		start = end
	}
	if strings.HasSuffix(text, "\n") {
		return lines
	}
	return lines
}

func lineTexts(lines []lineSpan) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = line.text
	}
	return out
}

func samePhaseShape(a []spec.Phase, b []spec.Phase) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Number != b[i].Number {
			return false
		}
	}
	return true
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

func renderReview(b *strings.Builder, review spec.ReviewState) {
	fmt.Fprintf(b, "## Review\n\nStatus: %s\nVerdict: %s\n\nFindings:\n", fallback(review.Status, "not_started"), fallback(review.Verdict, "none"))
	if len(review.Findings) == 0 {
		fmt.Fprintf(b, "- none\n\n")
		return
	}
	for _, finding := range review.Findings {
		fmt.Fprintf(b, "- [%s] `%s` %s\n", fallback(string(finding.Severity), string(corereview.SeverityNonBlocking)), fallback(finding.ID, "finding"), fallback(finding.Summary, "No summary recorded."))
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

func reviewFinding(id string, severity string, summary string) corereview.Finding {
	return corereview.Finding{ID: strings.TrimSpace(id), Severity: corereview.Severity(strings.TrimSpace(severity)), Summary: strings.TrimSpace(summary)}
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

func noneToEmpty(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "none") {
		return ""
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
	key, value, ok := parseSpecKeyValue(line)
	if !ok {
		return
	}
	switch normalizeSpecKey(key) {
	case "current phase":
		model.CurrentState.CurrentPhase = value
	case "next":
		model.CurrentState.Next = value
	case "reason":
		model.CurrentState.Reason = value
	case "blockers":
		model.CurrentState.Blockers = value
	case "allowed follow up", "allowed follow up command":
		model.CurrentState.AllowedFollowUp = value
	case "latest runner update":
		model.CurrentState.LatestRunnerUpdate = value
	case "review gate":
		model.CurrentState.ReviewGate = value
	}
}

func parseSpecListKeyValue(value string) (string, string, bool) {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "- ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	}
	return parseSpecKeyValue(trimmed)
}

func parseSpecKeyValue(value string) (string, string, bool) {
	key, body, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(key) == "" {
		return "", "", false
	}
	return strings.TrimSpace(key), cleanSpecValue(body), true
}

func cleanSpecValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	quote := value[0]
	if value[len(value)-1] != quote {
		return value
	}
	switch quote {
	case '"', '`':
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
	case '\'':
		return value[1 : len(value)-1]
	}
	return value
}

func normalizeSpecKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("_", " ", "-", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
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
