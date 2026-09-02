//   pkg/tools/memory.go

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/picoclaw/pkg/llm"
)

// MemoryTool provides a simple persistent key-value store per agent.
type MemoryTool struct {
	mu         sync.Mutex
	filePath   string
	data       map[string]interface{}
}

// NewMemoryTool creates a memory tool with the given storage file path.
func NewMemoryTool(filePath string) *MemoryTool {
	m := &MemoryTool{
		filePath: filePath,
		data:     make(map[string]interface{}),
	}
	// load existing data if file exists
	if b, err := os.ReadFile(filePath); err == nil {
		_ = json.Unmarshal(b, &m.data)
	}
	return m
}

func (t *MemoryTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action, _ := params["action"].(string)
	key, _ := params["key"].(string)
	if action == "" || key == "" {
		return nil, fmt.Errorf("both 'action' and 'key' are required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	switch action {
	case "set":
		value, ok := params["value"]
		if !ok {
			return nil, fmt.Errorf("'value' is required for action 'set'")
		}
		t.data[key] = value
		if err := t.save(); err != nil {
			return nil, err
		}
		return fmt.Sprintf("Stored %s", key), nil
	case "get":
		val, exists := t.data[key]
		if !exists {
			return nil, fmt.Errorf("key '%s' not found", key)
		}
		return val, nil
	case "delete":
		delete(t.data, key)
		if err := t.save(); err != nil {
			return nil, err
		}
		return fmt.Sprintf("Deleted %s", key), nil
	default:
		return nil, fmt.Errorf("unknown action '%s'", action)
	}
}

func (t *MemoryTool) save() error {
	b, err := json.MarshalIndent(t.data, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(t.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(t.filePath, b, 0644)
}

func (t *MemoryTool) Describe() llm.ToolDef {
	return llm.ToolDef{
		Name:        "memory",
		Description: "Store, retrieve, or delete key-value facts in the agent's persistent memory.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type": "string",
					"enum": []string{"get", "set", "delete"},
				},
				"key":   map[string]interface{}{"type": "string"},
				"value": map[string]interface{}{"type": "string"},
			},
			"required": []string{"action", "key"},
		},
	}
}