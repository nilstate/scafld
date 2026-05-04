package corebundle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
)

var agentDocFiles = []string{"AGENTS.md", "CLAUDE.md"}

// InitAgentDocs installs root agent docs, prepending a scafld section when needed.
func InitAgentDocs(ctx context.Context, root string) (Result, error) {
	return installAgentDocs(ctx, root, true)
}

// RefreshAgentDocs refreshes existing scafld sections in root agent docs.
func RefreshAgentDocs(ctx context.Context, root string) (Result, error) {
	return installAgentDocs(ctx, root, false)
}

func installAgentDocs(ctx context.Context, root string, prepend bool) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	var result Result
	for _, name := range agentDocFiles {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		body, err := assets.ReadFile("assets/agentdocs/" + name)
		if err != nil {
			return Result{}, fmt.Errorf("read embedded agent doc %s: %w", name, err)
		}
		section := canonicalSection(body)
		heading, err := sectionHeading(section)
		if err != nil {
			return Result{}, fmt.Errorf("agent doc %s: %w", name, err)
		}
		path := filepath.Join(root, name)
		if err := installAgentDoc(path, name, heading, section, prepend, &result); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

func installAgentDoc(path string, rel string, heading string, section []byte, prepend bool, result *Result) error {
	current, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := atomicfile.Write(path, section, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
		result.Created = append(result.Created, rel)
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", rel, err)
	}
	next, changed, err := mergeAgentDoc(current, heading, section, prepend)
	if err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	if !changed {
		result.Skipped = append(result.Skipped, rel)
		return nil
	}
	if err := atomicfile.Write(path, next, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	result.Updated = append(result.Updated, rel)
	return nil
}

func mergeAgentDoc(current []byte, heading string, section []byte, prepend bool) ([]byte, bool, error) {
	start, end, found, err := findTopLevelSection(current, heading)
	if err != nil {
		return nil, false, err
	}
	if found {
		existing := canonicalSection(current[start:end])
		if checksum(existing) == checksum(section) {
			return current, false, nil
		}
		next := append([]byte{}, current[:start]...)
		next = append(next, section...)
		if len(current[end:]) > 0 {
			if !bytes.HasSuffix(next, []byte("\n\n")) {
				next = append(next, '\n')
			}
			next = append(next, trimLeadingBlankLines(current[end:])...)
		}
		return next, true, nil
	}
	if !prepend {
		return current, false, nil
	}
	next := append([]byte{}, section...)
	if len(current) > 0 {
		next = append(next, '\n')
		if !bytes.HasPrefix(current, []byte("\n")) {
			next = append(next, '\n')
		}
		next = append(next, current...)
	}
	return next, true, nil
}

func findTopLevelSection(data []byte, heading string) (int, int, bool, error) {
	lines := splitMarkdownLines(data)
	inFence := false
	start := -1
	end := len(data)
	for _, line := range lines {
		trimmed := strings.TrimSpace(string(line.text))
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
		}
		if inFence || !strings.HasPrefix(trimmed, "# ") {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		if title == heading {
			if start >= 0 {
				return 0, 0, false, fmt.Errorf("duplicate section heading %q", heading)
			}
			start = line.offset
			continue
		}
		if start >= 0 {
			end = line.offset
			break
		}
	}
	if start < 0 {
		return 0, 0, false, nil
	}
	return start, end, true, nil
}

type markdownLine struct {
	offset int
	text   []byte
}

func splitMarkdownLines(data []byte) []markdownLine {
	var lines []markdownLine
	offset := 0
	for offset < len(data) {
		next := bytes.IndexByte(data[offset:], '\n')
		if next < 0 {
			lines = append(lines, markdownLine{offset: offset, text: data[offset:]})
			break
		}
		end := offset + next
		lines = append(lines, markdownLine{offset: offset, text: data[offset:end]})
		offset = end + 1
	}
	return lines
}

func sectionHeading(section []byte) (string, error) {
	for _, line := range strings.Split(string(section), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# ")), nil
		}
		if line != "" {
			break
		}
	}
	return "", fmt.Errorf("missing top-level heading")
}

func canonicalSection(data []byte) []byte {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return []byte{}
	}
	return []byte(trimmed + "\n")
}

func checksum(data []byte) [sha256.Size]byte {
	return sha256.Sum256(data)
}

func trimLeadingBlankLines(data []byte) []byte {
	for len(data) > 0 {
		switch data[0] {
		case '\n', '\r':
			data = data[1:]
		default:
			return data
		}
	}
	return data
}
