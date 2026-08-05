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

// LLMPipeline manages the interaction with the LLM.
type LLMPipeline struct {
	llmClient    llm.Client
	toolRegistry *tools.Registry
	logger       zerolog.Logger
	maxTokens    int // Maximum tokens for the LLM response
}

// LLMResponse encapsulates the result from the LLM.
type LLMResponse struct {
	Content      string
	ToolCalls    []ToolCall
	Reasoning    string // Placeholder for future use
	FinishReason string
}

// ToolCall represents a single tool invocation requested by the model.
type ToolCall struct {
	Name       string
	Parameters map[string]interface{}
}

// NewLLMPipeline creates a new pipeline with the given LLM client, tool registry,
// logger, and max tokens.
func NewLLMPipeline(client llm.Client, registry *tools.Registry, logger zerolog.Logger, maxTokens int) *LLMPipeline {
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	return &LLMPipeline{
		llmClient:    client,
		toolRegistry: registry,
		logger:       logger,
		maxTokens:    maxTokens,
	}
}

// Run executes the full pipeline.
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

	// 2: build base response
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
				if p.toolRegistry.Get(possibleCall.Name) != nil {
					response.ToolCalls = append(response.ToolCalls, ToolCall{
						Name:       possibleCall.Name,
						Parameters: possibleCall.Parameters,
					})
					response.Content = ""
					p.logger.Info().Str("tool", possibleCall.Name).Msg("Detected tool call from content JSON")
				} else {
					p.logger.Warn().Str("tool", possibleCall.Name).Msg("Ignored unrecognized tool call; clearing content to prevent raw JSON leak")
					response.Content = ""
				}
			}
		}
	}

	p.logger.Debug().Dur("duration", time.Since(start)).Int("tool_calls", len(response.ToolCalls)).Msg("LLM response processed")
	return response, nil
}