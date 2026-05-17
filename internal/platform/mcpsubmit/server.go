// Package mcpsubmit serves a minimal MCP stdio server for one structured payload.
package mcpsubmit

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
)

const protocolVersion = "2025-06-18"

// Accepted describes a canonical accepted submission.
type Accepted struct {
	Data              []byte
	Text              string
	StructuredContent map[string]any
}

// Options configures a single-tool submit server.
type Options struct {
	OutPath         string
	ServerName      string
	ToolName        string
	ToolTitle       string
	ToolDescription string
	SchemaJSON      string
	ParseAndEncode  func(string) (Accepted, error)
}

// Run serves a minimal MCP stdio server that accepts exactly one tool call and
// writes the canonical payload JSON to Options.OutPath.
func Run(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer, opts Options) error {
	opts.OutPath = strings.TrimSpace(opts.OutPath)
	if opts.OutPath == "" {
		return fmt.Errorf("--out is required")
	}
	if strings.TrimSpace(opts.ToolName) == "" {
		return fmt.Errorf("tool name is required")
	}
	if opts.ParseAndEncode == nil {
		return fmt.Errorf("submission parser is required")
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(opts.OutPath)), 0o755); err != nil {
		return fmt.Errorf("create submit dir: %w", err)
	}
	server := server{opts: opts, stdout: stdout, stderr: stderr}
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
	opts      Options
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
			"serverInfo":      map[string]any{"name": emptyDefault(s.opts.ServerName, "scafld-submit"), "version": "1.0.0"},
		})
	case "ping":
		s.respond(req.ID, map[string]any{})
	case "tools/list":
		s.respond(req.ID, map[string]any{"tools": []any{s.submitTool()}})
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
	if params.Name != s.opts.ToolName {
		s.respondError(req.ID, -32602, "unknown tool "+params.Name)
		return
	}
	if s.submitted {
		s.respond(req.ID, toolError(s.opts.ToolName+" was already called"))
		return
	}
	args := bytes.TrimSpace(params.Arguments)
	if len(args) == 0 {
		args = []byte("{}")
	}
	accepted, err := s.opts.ParseAndEncode(string(args))
	if err != nil {
		s.respond(req.ID, toolError(err.Error()))
		return
	}
	data := accepted.Data
	if len(data) == 0 && strings.TrimSpace(accepted.Text) != "" {
		data = []byte(accepted.Text)
	}
	if len(data) == 0 {
		s.respond(req.ID, toolError("empty encoded submission"))
		return
	}
	if err := writeAtomic(filepath.Clean(s.opts.OutPath), append(bytes.TrimSpace(data), '\n'), 0o600); err != nil {
		s.respond(req.ID, toolError("write submission: "+err.Error()))
		return
	}
	s.submitted = true
	contentText := accepted.Text
	if strings.TrimSpace(contentText) == "" {
		contentText = "Submission accepted."
	}
	structured := accepted.StructuredContent
	if structured == nil {
		structured = map[string]any{"ok": true}
	}
	s.respond(req.ID, map[string]any{
		"isError":           false,
		"content":           []any{map[string]any{"type": "text", "text": contentText}},
		"structuredContent": structured,
	})
}

func (s *server) submitTool() map[string]any {
	var inputSchema map[string]any
	if err := json.Unmarshal([]byte(s.opts.SchemaJSON), &inputSchema); err != nil {
		inputSchema = map[string]any{"type": "object"}
	}
	return map[string]any{
		"name":        s.opts.ToolName,
		"title":       emptyDefault(s.opts.ToolTitle, s.opts.ToolName),
		"description": strings.TrimSpace(s.opts.ToolDescription),
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
		fmt.Fprintf(s.stderr, "%s: marshal response: %v\n", emptyDefault(s.opts.ServerName, "scafld-submit"), err)
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
		fmt.Fprintf(s.stderr, "%s: marshal error: %v\n", emptyDefault(s.opts.ServerName, "scafld-submit"), err)
		return
	}
	fmt.Fprintln(s.stdout, string(data))
}

func emptyDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
