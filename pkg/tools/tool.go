//   pkg/tools/tool.go

package tools

import (
	"context"

	"github.com/picoclaw/pkg/llm"
)

// Tool defines a callable tool.
type Tool interface {
	Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)
	Describe() llm.ToolDef
}

// Registry holds available tools.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool.
func (r *Registry) Register(name string, t Tool) {
	r.tools[name] = t
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) Tool {
	return r.tools[name]
}

// List returns all registered tool names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	return names
}