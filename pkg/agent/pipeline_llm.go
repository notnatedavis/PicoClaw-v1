//   pkg/agent/pipeline_llm.go

//   Package agent provides the LLMPipeline that orchestrates calls to the LLM
//   and handles tool call detection (both from the native API and from JSON in the content)

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/picoclaw/pkg/llm"
	"github.com/picoclaw/pkg/tools"
	"github.com/rs/zerolog"
)

// LLMPipeline manages the interaction with the LLM: constructing the request,
// parsing the response, and detecting tool calls both from the API and from
// inline JSON that the model might generate
type LLMPipeline struct {
	llmClient    llm.Client
	toolRegistry *tools.Registry
	logger       zerolog.Logger
	maxTokens    int // Maximum tokens for the LLM response
}

// LLMResponse encapsulates the result from the LLM, including plain content,
// any tool calls, reasoning (if available), and the finish reason.
type LLMResponse struct {
	Content      string
	ToolCalls    []ToolCall
	Reasoning    string // Placeholder for future use (e.g., chain-of-thought)
	FinishReason string
}

// ToolCall represents a single tool invocation requested by the model.
type ToolCall struct {
	Name       string
	Parameters map[string]interface{}
}

// NewLLMPipeline creates a new pipeline with the given LLM client, tool registry,
// and logger. Default max tokens is set to 4096.
func NewLLMPipeline(client llm.Client, registry *tools.Registry, logger zerolog.Logger) *LLMPipeline {
	return &LLMPipeline{
		llmClient:    client,
		toolRegistry: registry,
		logger:       logger,
		maxTokens:    4096,
	}
}

// Run executes the full pipeline :
//  1. call the LLM with the provided messages and tool definitions
//  2. parse the response, extracting any tool calls from the official API field
//  3. if no official tool calls are found, attempt to detect a tool call in the
//     raw JSON content (as some models may output JSON instead of using the function-calling API)
//  4. validate that the detected tool exists in the registry; if not, discard it
//  5. return the parsed LLMResponse
func (p *LLMPipeline) Run(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (*LLMResponse, error) {
	start := time.Now()
	p.logger.Debug().Msg("Starting LLM pipeline run")

	// 1: call the LLM client
	resp, err := p.llmClient.Chat(ctx, &llm.ChatRequest{
		Messages:  messages,
		Tools:     tools,
		MaxTokens: p.maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// 2: vuild base response
	raw := resp.Content
	response := &LLMResponse{
		Content:      raw,
		FinishReason: resp.FinishReason,
	}

	// 3: extract tool calls from the official API response
	if resp.ToolCalls != nil {
		for _, tc := range resp.ToolCalls {
			response.ToolCalls = append(response.ToolCalls, ToolCall{
				Name:       tc.Function.Name,
				Parameters: tc.Function.Arguments,
			})
		}
	}

	// 4: if no official tool calls, try to detect a JSON tool call in the content
	if len(response.ToolCalls) == 0 {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
			var possibleCall struct {
				Name       string                 `json:"name"`
				Parameters map[string]interface{} `json:"parameters"`
			}
			if err := json.Unmarshal([]byte(trimmed), &possibleCall); err == nil && possibleCall.Name != "" {
				// verify that the tool exists in the registry.
				if p.toolRegistry.Get(possibleCall.Name) != nil {
					response.ToolCalls = append(response.ToolCalls, ToolCall{
						Name:       possibleCall.Name,
						Parameters: possibleCall.Parameters,
					})
					// clear the content to prevent raw JSON from being shown to the user.
					response.Content = ""
					p.logger.Info().
						Str("tool", possibleCall.Name).
						Msg("Detected tool call from content JSON")
				} else {
					// tool not available – discard raw JSON to avoid leaking it to the user.
					p.logger.Warn().
						Str("tool", possibleCall.Name).
						Msg("Ignored unrecognized tool call; clearing content to prevent raw JSON leak")
					response.Content = ""
				}
			}
		}
	}

	// 5: log metrics and return
	p.logger.Debug().
		Dur("duration", time.Since(start)).
		Int("tool_calls", len(response.ToolCalls)).
		Msg("LLM response processed")

	return response, nil
}