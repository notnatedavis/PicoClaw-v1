// pkg/llm/types.go
package llm

import "context"

// Message represents a single chat message.
type Message struct {
	Role    string // "system", "user", "assistant", "tool"
	Content string
	Name    string // optional, for tool messages
}

// ToolDef describes a tool available to the model.
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]interface{} // JSON Schema for the tool arguments
}

// ChatRequest holds parameters for an LLM chat call.
type ChatRequest struct {
	Messages  []Message
	Tools     []ToolDef
	MaxTokens int
}

// ToolCall is a request from the model to call a tool.
type ToolCall struct {
	Function struct {
		Name      string
		Arguments map[string]interface{}
	}
}

// ChatResponse is the model's reply.
type ChatResponse struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
}

// Client is the interface for an LLM provider.
type Client interface {
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
}