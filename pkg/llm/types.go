//   pkg/llm/types.go

//   Package llm defines core types and interfaces for interacting with
//   Large Language Model providers
package llm

import "context"

// Message represents a single chat message in a conversation
type Message struct {
	Role    string // "system", "user", "assistant", "tool"
	Content string
	Name    string // Optional, used for tool messages
}

// ToolDef describes a tool that the LLM can invoke, following JSON Schema
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]interface{} // JSON Schema for the tool arguments
}

// ChatRequest groups all parameters for a chat completion call
type ChatRequest struct {
	Messages  []Message
	Tools     []ToolDef
	MaxTokens int
}

// ToolCall represents a tool invocation requested by the LLM
type ToolCall struct {
	Function struct {
		Name      string
		Arguments map[string]interface{}
	}
}

// ChatResponse is the result of a chat completion
type ChatResponse struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
}

// Client is the interface that all LLM providers must implement
type Client interface {
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
}