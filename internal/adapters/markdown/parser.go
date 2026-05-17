package markdown

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

var (
	phaseHeadingPattern  = regexp.MustCompile(`^## Phase ([0-9]+):\s*(.+?)\s*$`)
	criterionLinePattern = regexp.MustCompile(`^- \[[ xX]\] ` + "`" + `([^` + "`" + `]+)` + "`" + `\s+([^-]+?)\s+-\s*(.*)$`)
	findingLinePattern   = regexp.MustCompile(`^- \[([a-z]+)/(blocks completion|non-blocking)\]\s+` + "`" + `([^` + "`" + `]+)` + "`" + `\s+(.*)$`)
)

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
	model           spec.Model
	section         string
	phase           *spec.Phase
	criterion       *spec.Criterion
	reviewFinding   *corereview.Finding
	hardenRound     *spec.HardenRound
	hardenCheck     *spec.HardenCheck
	hardenQuestion  *spec.HardenQuestion
	hardenObjection *spec.HardenDesignObjection
	hardenEdit      *spec.HardenRecommendedEdit
	phaseField      string
	hardenField     string
	listTarget      *([]string)
	phaseIDs        map[string]bool
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
	p.reviewFinding = nil
	p.hardenRound = nil
	p.hardenCheck = nil
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
	p.reviewFinding = nil
	p.hardenRound = nil
	p.hardenCheck = nil
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
		case "mode":
			p.model.Review.Mode = corereview.Mode(value)
			return true
		case "provider":
			p.model.Review.Provider = value
			if provider, model, ok := strings.Cut(value, ":"); ok {
				p.model.Review.Provider = strings.TrimSpace(provider)
				p.model.Review.Model = strings.TrimSpace(model)
			}
			return true
		case "output":
			p.model.Review.OutputFormat = value
			return true
		case "normalizations":
			p.model.Review.Normalizations = splitCSV(value)
			return true
		case "summary":
			p.model.Review.Summary = value
			return true
		case "findings":
			return true
		}
	}
	if match := findingLinePattern.FindStringSubmatch(line); match != nil {
		p.model.Review.Findings = append(p.model.Review.Findings, reviewFinding(match[3], match[1], match[2], match[4]))
		p.reviewFinding = &p.model.Review.Findings[len(p.model.Review.Findings)-1]
		return true
	}
	if p.reviewFinding != nil {
		if key, value, ok := parseIndentedSpecKeyValue(line); ok {
			switch normalizeSpecKey(key) {
			case "location":
				path, lineNo := parseLocation(value)
				p.reviewFinding.Location = &corereview.Location{Path: path, Line: lineNo}
			case "evidence":
				p.reviewFinding.Evidence = value
			case "impact":
				p.reviewFinding.Impact = value
			case "validation":
				p.reviewFinding.Validation = value
			}
			return true
		}
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
		case "verdict":
			if p.hardenRound != nil {
				p.hardenRound.Verdict = value
			}
			return true
		case "provider":
			if p.hardenRound != nil {
				p.hardenRound.Provider = value
			}
			return true
		case "model":
			if p.hardenRound != nil {
				p.hardenRound.Model = value
			}
			return true
		case "output format":
			if p.hardenRound != nil {
				p.hardenRound.OutputFormat = value
			}
			return true
		case "summary":
			if p.hardenRound != nil {
				p.hardenRound.Summary = value
			}
			return true
		case "questions":
			p.hardenField = "questions"
			p.hardenCheck = nil
			p.hardenObjection = nil
			p.hardenEdit = nil
			return true
		case "checks":
			p.hardenField = "checks"
			p.hardenQuestion = nil
			p.hardenObjection = nil
			p.hardenEdit = nil
			return true
		case "design objections":
			p.hardenField = "design_objections"
			p.hardenCheck = nil
			p.hardenQuestion = nil
			p.hardenEdit = nil
			return true
		case "recommended edits":
			p.hardenField = "recommended_edits"
			p.hardenCheck = nil
			p.hardenQuestion = nil
			p.hardenObjection = nil
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
	p.hardenCheck = nil
	p.hardenQuestion = nil
	p.hardenObjection = nil
	p.hardenEdit = nil
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
		return true
	}
	if p.hardenField == "checks" {
		if key, body, ok := parseSpecKeyValue(value); ok && normalizeSpecKey(key) == "check" {
			p.startHardenCheck(body)
			return true
		}
		p.startHardenCheck(value)
		return true
	}
	if p.hardenField == "design_objections" {
		p.startHardenObjection(value)
		return true
	}
	if p.hardenField == "recommended_edits" {
		p.startHardenEdit(value)
		return true
	}
	return true
}

func (p *parser) startHardenCheck(value string) {
	if p.hardenRound == nil {
		return
	}
	value = cleanSpecValue(value)
	if value == "" {
		return
	}
	normalized := normalizeHardenQuestion(value)
	for i := range p.hardenRound.Checks {
		if normalizeHardenQuestion(p.hardenRound.Checks[i].Name) == normalized {
			p.hardenCheck = &p.hardenRound.Checks[i]
			return
		}
	}
	p.hardenRound.Checks = append(p.hardenRound.Checks, spec.HardenCheck{Name: value})
	p.hardenCheck = &p.hardenRound.Checks[len(p.hardenRound.Checks)-1]
	p.hardenQuestion = nil
	p.hardenObjection = nil
	p.hardenEdit = nil
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
	p.hardenCheck = nil
	p.hardenObjection = nil
	p.hardenEdit = nil
}

func (p *parser) startHardenObjection(value string) {
	if p.hardenRound == nil {
		return
	}
	value = cleanSpecValue(value)
	if value == "" {
		return
	}
	objection := spec.HardenDesignObjection{Summary: value}
	if strings.HasPrefix(value, "`") {
		if rest := strings.TrimPrefix(value, "`"); rest != value {
			if id, tail, ok := strings.Cut(rest, "`"); ok {
				objection.ID = strings.TrimSpace(id)
				tail = strings.TrimSpace(tail)
				if severity, summary, ok := strings.Cut(tail, " - "); ok {
					objection.Severity = strings.TrimSpace(severity)
					objection.Summary = strings.TrimSpace(summary)
				} else if tail != "" {
					objection.Summary = strings.TrimSpace(tail)
				}
			}
		}
	}
	p.hardenRound.DesignObjections = append(p.hardenRound.DesignObjections, objection)
	p.hardenObjection = &p.hardenRound.DesignObjections[len(p.hardenRound.DesignObjections)-1]
	p.hardenCheck = nil
	p.hardenQuestion = nil
	p.hardenEdit = nil
}

func (p *parser) startHardenEdit(value string) {
	if p.hardenRound == nil {
		return
	}
	value = cleanSpecValue(value)
	if value == "" {
		return
	}
	p.hardenRound.RecommendedEdits = append(p.hardenRound.RecommendedEdits, spec.HardenRecommendedEdit{Section: value})
	p.hardenEdit = &p.hardenRound.RecommendedEdits[len(p.hardenRound.RecommendedEdits)-1]
	p.hardenCheck = nil
	p.hardenQuestion = nil
	p.hardenObjection = nil
}

func (p *parser) handleHardenQuestionDetail(value string) bool {
	key, body, ok := parseSpecKeyValue(value)
	if !ok {
		return true
	}
	if p.hardenField == "checks" {
		if normalizeSpecKey(key) == "check" {
			p.startHardenCheck(body)
			return true
		}
		if p.hardenCheck == nil {
			return true
		}
		switch normalizeSpecKey(key) {
		case "grounded in":
			p.hardenCheck.GroundedIn = body
		case "result":
			p.hardenCheck.Result = body
		case "evidence":
			p.hardenCheck.Evidence = body
		}
		return true
	}
	if p.hardenField == "design_objections" {
		if p.hardenObjection == nil {
			return true
		}
		switch normalizeSpecKey(key) {
		case "grounded in":
			p.hardenObjection.GroundedIn = body
		case "evidence":
			p.hardenObjection.Evidence = body
		case "recommendation":
			p.hardenObjection.Recommendation = body
		}
		return true
	}
	if p.hardenField == "recommended_edits" {
		if p.hardenEdit == nil {
			return true
		}
		switch normalizeSpecKey(key) {
		case "grounded in":
			p.hardenEdit.GroundedIn = body
		case "recommendation":
			p.hardenEdit.Recommendation = body
		}
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
	if p.listTarget == nil {
		return false
	}
	if !strings.HasPrefix(line, "- ") {
		return p.handleListContinuation(line)
	}
	value := strings.TrimSpace(strings.TrimPrefix(line, "- "))
	if value != "none" && value != "" {
		*p.listTarget = append(*p.listTarget, value)
	}
	return true
}

func (p *parser) handleListContinuation(line string) bool {
	if p.listTarget == nil || len(*p.listTarget) == 0 || !strings.HasPrefix(line, "  ") {
		return false
	}
	value := strings.TrimSpace(line)
	if value == "" || strings.HasPrefix(value, "- ") {
		return false
	}
	items := *p.listTarget
	items[len(items)-1] = strings.TrimSpace(items[len(items)-1] + " " + value)
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
	if p.phaseField == "changes" && p.appendPhaseChangeContinuation(line) {
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

func (p *parser) appendPhaseChangeContinuation(line string) bool {
	if p.phase == nil || len(p.phase.Changes) == 0 || !strings.HasPrefix(line, "  ") {
		return false
	}
	value := strings.TrimSpace(line)
	if value == "" || strings.HasPrefix(value, "- ") {
		return false
	}
	p.phase.Changes[len(p.phase.Changes)-1] = strings.TrimSpace(p.phase.Changes[len(p.phase.Changes)-1] + " " + value)
	return true
}

func (p *parser) handleCriterionStart(line string) bool {
	if p.phase != nil {
		if p.phaseField != "acceptance" {
			return false
		}
	} else if p.section != "acceptance" {
		return false
	}
	match := criterionLinePattern.FindStringSubmatch(line)
	if match == nil {
		return false
	}
	criterionType := strings.TrimSpace(match[2])
	expectedKind := acceptance.ExpectedExitCodeZero
	if criterionType == "browser" {
		expectedKind = acceptance.ExpectedBrowserEvidence
	}
	c := spec.Criterion{ID: match[1], Type: criterionType, Title: strings.TrimSpace(match[3]), ExpectedKind: expectedKind, Status: "pending"}
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

func parseIndentedSpecKeyValue(value string) (string, string, bool) {
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

func parseLocation(value string) (string, int) {
	value = strings.Trim(strings.TrimSpace(value), "`")
	if value == "" {
		return "", 0
	}
	path, lineText, ok := strings.Cut(value, ":")
	if !ok {
		return value, 0
	}
	line, err := strconv.Atoi(strings.TrimSpace(lineText))
	if err != nil || line < 1 {
		return value, 0
	}
	return strings.TrimSpace(path), line
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

func noneToEmpty(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "none") {
		return ""
	}
	return value
}

func reviewFinding(id string, severity string, blocking string, summary string) corereview.Finding {
	return corereview.Finding{ID: strings.TrimSpace(id), Severity: corereview.Severity(strings.TrimSpace(severity)), BlocksCompletion: strings.TrimSpace(blocking) == "blocks completion", Status: corereview.FindingOpen, Summary: strings.TrimSpace(summary)}
}
