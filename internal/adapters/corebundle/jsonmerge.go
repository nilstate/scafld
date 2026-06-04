package corebundle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

const (
	mcpServersKey      = "mcpServers"
	claudeHooksKey     = "hooks"
	claudeStopHookKey  = "Stop"
	scafldStopHookName = "finalize-stop"
)

// MergeMCPConfig upserts the scafld MCP server while preserving unrelated config.
func MergeMCPConfig(current []byte, desired []byte) ([]byte, bool, bool, error) {
	currentObject, currentExists, err := parseJSONObject(current, ".mcp.json")
	if err != nil {
		return nil, false, false, err
	}
	desiredObject, _, err := parseJSONObject(desired, "embedded .mcp.json")
	if err != nil {
		return nil, false, false, err
	}
	desiredServers, err := objectField(desiredObject, mcpServersKey, "embedded .mcp.json")
	if err != nil {
		return nil, false, false, err
	}
	if len(desiredServers) == 0 {
		return nil, false, false, fmt.Errorf("embedded .mcp.json has no %s entries", mcpServersKey)
	}
	nextObject := cloneObject(currentObject)
	servers, err := objectField(nextObject, mcpServersKey, ".mcp.json")
	if err != nil {
		return nil, false, false, err
	}
	if servers == nil {
		servers = map[string]any{}
	}
	conflict := false
	for name, desiredServer := range desiredServers {
		if currentServer, ok := servers[name]; ok && !reflect.DeepEqual(currentServer, desiredServer) {
			conflict = true
		}
		servers[name] = desiredServer
	}
	nextObject[mcpServersKey] = servers
	next, err := marshalJSONObject(nextObject)
	if err != nil {
		return nil, false, false, err
	}
	if !currentExists {
		return next, true, false, nil
	}
	currentCanonical, err := marshalJSONObject(currentObject)
	if err != nil {
		return nil, false, false, err
	}
	return next, !bytes.Equal(currentCanonical, next), conflict, nil
}

// MergeClaudeSettings upserts the scafld Stop hook while preserving unrelated settings.
func MergeClaudeSettings(current []byte, desired []byte) ([]byte, bool, bool, error) {
	currentObject, currentExists, err := parseJSONObject(current, ".claude/settings.json")
	if err != nil {
		return nil, false, false, err
	}
	desiredObject, _, err := parseJSONObject(desired, "embedded .claude/settings.json")
	if err != nil {
		return nil, false, false, err
	}
	desiredStop, err := stopHooks(desiredObject, "embedded .claude/settings.json")
	if err != nil {
		return nil, false, false, err
	}
	desiredHook, ok := findNamedHook(desiredStop, scafldStopHookName)
	if !ok {
		return nil, false, false, fmt.Errorf("embedded .claude/settings.json missing %q Stop hook", scafldStopHookName)
	}

	nextObject := cloneObject(currentObject)
	hooks, err := objectField(nextObject, claudeHooksKey, ".claude/settings.json")
	if err != nil {
		return nil, false, false, err
	}
	if hooks == nil {
		hooks = map[string]any{}
	}
	currentStop, err := arrayField(hooks, claudeStopHookKey, ".claude/settings.json hooks")
	if err != nil {
		return nil, false, false, err
	}
	mergedStop, conflict := upsertNamedHook(currentStop, desiredHook, scafldStopHookName)
	hooks[claudeStopHookKey] = mergedStop
	nextObject[claudeHooksKey] = hooks

	next, err := marshalJSONObject(nextObject)
	if err != nil {
		return nil, false, false, err
	}
	if !currentExists {
		return next, true, false, nil
	}
	currentCanonical, err := marshalJSONObject(currentObject)
	if err != nil {
		return nil, false, false, err
	}
	return next, !bytes.Equal(currentCanonical, next), conflict, nil
}

func parseJSONObject(data []byte, label string) (map[string]any, bool, error) {
	if strings.TrimSpace(string(data)) == "" {
		return map[string]any{}, false, nil
	}
	var object map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&object); err != nil {
		return nil, false, fmt.Errorf("decode %s: %w", label, err)
	}
	if object == nil {
		return nil, false, fmt.Errorf("%s must be a JSON object", label)
	}
	return object, true, nil
}

func objectField(object map[string]any, key string, label string) (map[string]any, error) {
	value, ok := object[key]
	if !ok || value == nil {
		return nil, nil
	}
	nested, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s field %q must be a JSON object", label, key)
	}
	return nested, nil
}

func arrayField(object map[string]any, key string, label string) ([]any, error) {
	value, ok := object[key]
	if !ok || value == nil {
		return nil, nil
	}
	array, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s field %q must be a JSON array", label, key)
	}
	return array, nil
}

func stopHooks(object map[string]any, label string) ([]any, error) {
	hooks, err := objectField(object, claudeHooksKey, label)
	if err != nil {
		return nil, err
	}
	if hooks == nil {
		return nil, nil
	}
	return arrayField(hooks, claudeStopHookKey, label+" hooks")
}

func findNamedHook(hooks []any, name string) (any, bool) {
	for _, hook := range hooks {
		hookObject, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		if hookObject["name"] == name {
			return hook, true
		}
	}
	return nil, false
}

func upsertNamedHook(hooks []any, desired any, name string) ([]any, bool) {
	out := make([]any, 0, len(hooks)+1)
	found := false
	conflict := false
	for _, hook := range hooks {
		hookObject, ok := hook.(map[string]any)
		if ok && hookObject["name"] == name {
			if !reflect.DeepEqual(hook, desired) {
				conflict = true
			}
			if !found {
				out = append(out, desired)
				found = true
			}
			continue
		}
		out = append(out, hook)
	}
	if !found {
		out = append(out, desired)
	}
	return out, conflict
}

func cloneObject(object map[string]any) map[string]any {
	out := make(map[string]any, len(object))
	for key, value := range object {
		out[key] = cloneJSONValue(value)
	}
	return out
}

func cloneJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneObject(typed)
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = cloneJSONValue(value)
		}
		return out
	default:
		return typed
	}
}

func marshalJSONObject(object map[string]any) ([]byte, error) {
	ordered := orderJSONValue(object)
	data, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func orderJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(orderedObject, 0, len(keys))
		for _, key := range keys {
			out = append(out, orderedMember{Key: key, Value: orderJSONValue(typed[key])})
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = orderJSONValue(value)
		}
		return out
	default:
		return typed
	}
}

type orderedMember struct {
	Key   string
	Value any
}

type orderedObject []orderedMember

func (o orderedObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, member := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(member.Key)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(member.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
