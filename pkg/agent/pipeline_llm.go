//   pkg/agent/pipeline_llm.go

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

type LLMPipeline struct {
	llmClient    llm.Client
	toolRegistry *tools.Registry
	logger       zerolog.Logger
	maxTokens    int
}

type LLMResponse struct {
	Content      string
	ToolCalls    []ToolCall
	Reasoning    string
	FinishReason string
}

type ToolCall struct {
	Name       string
	Parameters map[string]interface{}
}

func NewLLMPipeline(client llm.Client, registry *tools.Registry, logger zerolog.Logger) *LLMPipeline {
	return &LLMPipeline{
		llmClient:    client,
		toolRegistry: registry,
		logger:       logger,
		maxTokens:    4096,
	}
}

func (p *LLMPipeline) Run(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (*LLMResponse, error) {
	start := time.Now()
	resp, err := p.llmClient.Chat(ctx, &llm.ChatRequest{
		Messages:  messages,
		Tools:     tools,
		MaxTokens: p.maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	raw := resp.Content
	response := &LLMResponse{
		Content:      raw,
		FinishReason: resp.FinishReason,
	}

	// tool call extraction
	if resp.ToolCalls != nil {
		for _, tc := range resp.ToolCalls {
			response.ToolCalls = append(response.ToolCalls, ToolCall{
				Name:       tc.Function.Name,
				Parameters: tc.Function.Arguments,
			})
		}
	}

	// if no tool calls were found, try to detect a JSON tool call in the content.
	if len(response.ToolCalls) == 0 {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
			var possibleCall struct {
				Name       string                 `json:"name"`
				Parameters map[string]interface{} `json:"parameters"`
			}
			if err := json.Unmarshal([]byte(trimmed), &possibleCall); err == nil && possibleCall.Name != "" {
				// Verify tool exists in registry
				if p.toolRegistry.Get(possibleCall.Name) != nil {
					response.ToolCalls = append(response.ToolCalls, ToolCall{
						Name:       possibleCall.Name,
						Parameters: possibleCall.Parameters,
					})
					response.Content = "" // clear raw content to avoid duplicate answer
					p.logger.Info().
						Str("tool", possibleCall.Name).
						Msg("Detected tool call from content JSON")
				} else {
					// Tool not available – discard the raw JSON so it isn't echoed to the user.
					p.logger.Warn().
						Str("tool", possibleCall.Name).
						Msg("Ignored unrecognized tool call; clearing content to prevent raw JSON leak")
					response.Content = ""
				}
			}
		}
	}

	p.logger.Debug().
		Dur("duration", time.Since(start)).
		Int("tool_calls", len(response.ToolCalls)).
		Msg("LLM response processed")

	return response, nil
}