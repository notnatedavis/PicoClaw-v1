//   pkg/llm/openai_client.go

//   Package llm provides a concrete OpenAI‑compatible client that implements Client.
//   It can be used with Ollama's /v1 endpoint or any service that mimics the OpenAI API.

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// openaiClient implements Client using an OpenAI‑compatible HTTP endpoint.
type openaiClient struct {
	baseURL    string
	apiKey     string // optional, left empty for local services
	model      string
	httpClient *http.Client
	logger     zerolog.Logger
}

// NewOpenAIClient creates a new OpenAI‑compatible client.
// baseURL should include the /v1 suffix, e.g. "http://127.0.0.1:11434/v1".
// model is the model name to use.
func NewOpenAIClient(baseURL, model string, logger zerolog.Logger) (Client, error) {
	return &openaiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}, nil
}

// Chat sends a chat completion request and parses the response.
func (c *openaiClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Build OpenAI‑style request body
	openaiReq := struct {
		Model     string    `json:"model"`
		Messages  []Message `json:"messages"`
		Tools     []Tool    `json:"tools,omitempty"`
		MaxTokens int       `json:"max_tokens,omitempty"`
	}{
		Model:     c.model,
		Messages:  req.Messages,
		MaxTokens: req.MaxTokens,
	}

	// Map tools to OpenAI tool format
	for _, t := range req.Tools {
		openaiReq.Tools = append(openaiReq.Tools, Tool{
			Type: "function",
			Function: FunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse OpenAI chat completion response
	var openaiResp struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string                 `json:"name"`
						Arguments string                 `json:"arguments"` // JSON string
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}
	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := openaiResp.Choices[0]
	chatResp := &ChatResponse{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
	}

	for _, tc := range choice.Message.ToolCalls {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			c.logger.Warn().Err(err).Str("tool", tc.Function.Name).Msg("Failed to parse tool arguments JSON")
			args = map[string]interface{}{}
		}
		chatResp.ToolCalls = append(chatResp.ToolCalls, ToolCall{
			Function: struct {
				Name      string
				Arguments map[string]interface{}
			}{
				Name:      tc.Function.Name,
				Arguments: args,
			},
		})
	}
	return chatResp, nil
}

// --- OpenAI API types (internal) ---
type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}