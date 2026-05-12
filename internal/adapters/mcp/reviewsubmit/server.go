package reviewsubmit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
)

const (
	protocolVersion = "2025-06-18"
	toolName        = "submit_review"
)

// Run serves a minimal MCP stdio server that accepts exactly one
// submit_review tool call and writes the canonical ReviewDossier JSON to outPath.
func Run(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer, outPath string) error {
	outPath = strings.TrimSpace(outPath)
	if outPath == "" {
		return fmt.Errorf("--out is required")
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(outPath)), 0o755); err != nil {
		return fmt.Errorf("create submit dir: %w", err)
	}
	server := server{outPath: filepath.Clean(outPath), stdout: stdout, stderr: stderr}
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 1024), 16*1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		server.handle(line)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read MCP stdio: %w", err)
	}
	return ctx.Err()
}

type server struct {
	outPath   string
	stdout    io.Writer
	stderr    io.Writer
	submitted bool
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *server) handle(line string) {
	var req rpcRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		s.respondError(json.RawMessage("null"), -32700, "parse error")
		return
	}
	if len(req.ID) == 0 {
		return
	}
	switch req.Method {
	case "initialize":
		s.respond(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "scafld-review-submit", "version": "1.0.0"},
		})
	case "ping":
		s.respond(req.ID, map[string]any{})
	case "tools/list":
		s.respond(req.ID, map[string]any{"tools": []any{submitReviewTool()}})
	case "tools/call":
		s.handleToolCall(req)
	default:
		s.respondError(req.ID, -32601, "method not found")
	}
}

func (s *server) handleToolCall(req rpcRequest) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.respondError(req.ID, -32602, "invalid tools/call params")
		return
	}
	if params.Name != toolName {
		s.respondError(req.ID, -32602, "unknown tool "+params.Name)
		return
	}
	if s.submitted {
		s.respond(req.ID, toolError("submit_review was already called"))
		return
	}
	args := bytes.TrimSpace(params.Arguments)
	if len(args) == 0 {
		args = []byte("{}")
	}
	dossier, err := review.ParseText(string(args))
	if err != nil {
		s.respond(req.ID, toolError(err.Error()))
		return
	}
	data := []byte(review.EncodeDossier(dossier))
	if len(data) == 0 {
		s.respond(req.ID, toolError("empty encoded ReviewDossier"))
		return
	}
	if err := atomicfile.Write(s.outPath, append(data, '\n'), 0o600); err != nil {
		s.respond(req.ID, toolError("write ReviewDossier: "+err.Error()))
		return
	}
	s.submitted = true
	s.respond(req.ID, map[string]any{
		"isError": false,
		"content": []any{map[string]any{"type": "text", "text": "ReviewDossier accepted."}},
		"structuredContent": map[string]any{
			"ok":      true,
			"verdict": dossier.Verdict,
		},
	})
}

func submitReviewTool() map[string]any {
	var inputSchema map[string]any
	if err := json.Unmarshal([]byte(review.DossierSchemaJSON()), &inputSchema); err != nil {
		inputSchema = map[string]any{"type": "object"}
	}
	return map[string]any{
		"name":        toolName,
		"title":       "Submit scafld review",
		"description": "Submit the final scafld ReviewDossier. Call exactly once after completing the read-only adversarial review.",
		"inputSchema": inputSchema,
	}
}

func toolError(message string) map[string]any {
	return map[string]any{
		"isError": true,
		"content": []any{map[string]any{"type": "text", "text": message}},
	}
}

func (s *server) respond(id json.RawMessage, result any) {
	if len(id) == 0 {
		return
	}
	data, err := json.Marshal(rpcResponse{JSONRPC: "2.0", ID: id, Result: result})
	if err != nil {
		fmt.Fprintf(s.stderr, "scafld review-submit-stdio: marshal response: %v\n", err)
		return
	}
	fmt.Fprintln(s.stdout, string(data))
}

func (s *server) respondError(id json.RawMessage, code int, message string) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	data, err := json.Marshal(rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}})
	if err != nil {
		fmt.Fprintf(s.stderr, "scafld review-submit-stdio: marshal error: %v\n", err)
		return
	}
	fmt.Fprintln(s.stdout, string(data))
}
