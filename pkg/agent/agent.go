//   pkg/agent/agent.go

//   Package agent provides the core AI agent implementation, including session management,
//   message processing, tool calling, and conversational history.

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

// Session holds the conversational state for a single agent interaction.
// The Memory map stores arbitrary data, including a "history" slice of llm.Message.
type Session struct {
	ID     string                 // unique session identifier (e.g., chat_id)
	Memory map[string]interface{} // simple key-value store for ephemeral memory
}

// AgentConfig carries the initialisation parameters for creating an Agent.
type AgentConfig struct {
	ID           string   // unique agent identifier (lowercase)
	Name         string   // human-readable name
	Model        string   // LLM model name (e.g., "llama3.2:3b")
	SystemPrompt string   // system prompt that defines the agent's persona
	Tools        []string // list of tool names this agent is allowed to use
	Workspace    string   // working directory for file operations
	MaxTokens    int      // maximum tokens for LLM responses
}

// Agent is a self-contained AI agent with its own configuration, LLM client,
// tool registry, and pipeline. processes user messages and may invoke tools.
type Agent struct {
	ID           string
	Name         string
	Model        string
	SystemPrompt string
	Tools        []string
	Workspace    string

	llmClient          llm.Client
	toolRegistry       *tools.Registry
	pipeline           *LLMPipeline
	logger             zerolog.Logger
	registry           *Registry // reference to the global agent registry (for future cross-agent calls)
	reviewAgent        *Agent    // internal review agent (may be nil)
	maxReviewIterations int      // maximum number of review loops (0 = disabled)
}

// NewAgent creates and fully initialises a new Agent instance.
func NewAgent(cfg AgentConfig, llmClient llm.Client, toolReg *tools.Registry, logger zerolog.Logger, agentReg *Registry) *Agent {
	pipeline := NewLLMPipeline(llmClient, toolReg, logger, cfg.MaxTokens)
	return &Agent{
		ID:           cfg.ID,
		Name:         cfg.Name,
		Model:        cfg.Model,
		SystemPrompt: cfg.SystemPrompt,
		Tools:        cfg.Tools,
		Workspace:    cfg.Workspace,
		llmClient:    llmClient,
		toolRegistry: toolReg,
		pipeline:     pipeline,
		logger:       logger.With().Str("agent_id", cfg.ID).Logger(),
		registry:     agentReg,
	}
}

// SetReviewAgent sets the internal review agent used for output validation.
func (a *Agent) SetReviewAgent(review *Agent) {
	a.reviewAgent = review
}

// SetMaxReviewIterations sets the maximum number of review iterations (0 disables review).
func (a *Agent) SetMaxReviewIterations(n int) {
	if n < 0 {
		n = 0
	}
	a.maxReviewIterations = n
}

// Process is the public entry point for processing a user message within a session.
// It runs the main agent, optionally validates the answer with a review agent,
// and repeats generation until acceptable or iteration limit reached.
func (a *Agent) Process(ctx context.Context, userMsg string, session *Session) (string, error) {
	// Build initial messages: system + history + user
	messages := a.buildMessages(session, userMsg)
	toolDefs := a.getToolDefs()

	for attempt := 0; attempt <= a.maxReviewIterations; attempt++ {
		// Generate answer using the LLM and tools (no history update yet)
		finalAnswer, err := a.generateAnswer(ctx, messages, toolDefs)
		if err != nil {
			return "", fmt.Errorf("LLM generation error: %w", err)
		}

		// If no review agent or last allowed attempt, accept the answer
		if a.reviewAgent == nil || attempt == a.maxReviewIterations {
			a.updateHistory(session, userMsg, finalAnswer)
			return finalAnswer, nil
		}

		// Review the answer
		acceptable, feedback, err := a.reviewAgent.evaluateAnswer(ctx, userMsg, a.getHistoryFromSession(session), finalAnswer)
		if err != nil {
			// Fallback: accept the answer if review fails
			a.logger.Warn().Err(err).Msg("Review evaluation failed; accepting answer as fallback")
			a.updateHistory(session, userMsg, finalAnswer)
			return finalAnswer, nil
		}

		if acceptable {
			a.updateHistory(session, userMsg, finalAnswer)
			return finalAnswer, nil
		}

		// Not acceptable: add feedback as a new user message for regeneration
		a.logger.Info().Int("attempt", attempt+1).Str("feedback", feedback).Msg("Answer rejected by reviewer, regenerating")
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: fmt.Sprintf("Your previous response was not satisfactory. Feedback: %s\nPlease provide a revised answer.", feedback),
		})
	}

	// Should never reach here if maxReviewIterations >= 0 and handled above
	return "", fmt.Errorf("review loop exhausted without resolution")
}

// generateAnswer performs the LLM call(s) and tool execution to produce a final answer.
// It does not modify session history.
func (a *Agent) generateAnswer(ctx context.Context, messages []llm.Message, toolDefs []llm.ToolDef) (string, error) {
	resp, err := a.pipeline.Run(ctx, messages, toolDefs)
	if err != nil {
		return "", err
	}

	finalAnswer := resp.Content
	if len(resp.ToolCalls) > 0 {
		// Execute all tool calls and collect results
		var toolResults []string
		for _, tc := range resp.ToolCalls {
			tool := a.toolRegistry.Get(tc.Name)
			if tool == nil {
				toolResults = append(toolResults, fmt.Sprintf("Tool %s not available", tc.Name))
				continue
			}
			result, err := tool.Execute(ctx, tc.Parameters)
			if err != nil {
				toolResults = append(toolResults, fmt.Sprintf("Error executing %s: %v", tc.Name, err))
				continue
			}
			toolResults = append(toolResults, fmt.Sprintf("Tool %s result: %v", tc.Name, result))
			a.logger.Info().Str("tool", tc.Name).Interface("result", result).Msg("Tool executed")
		}

		// Build a follow-up message with tool results and ask LLM for a summary
		combined := strings.Join(toolResults, "\n")
		followUpMsg := llm.Message{Role: "user", Content: fmt.Sprintf("Tool execution results:\n%s\n\nPlease summarise these results in a natural, concise reply to the user.", combined)}
		messages = append(messages, followUpMsg)

		finalResp, err := a.pipeline.Run(ctx, messages, toolDefs)
		if err != nil {
			// fallback: return raw tool results as answer
			a.logger.Warn().Err(err).Msg("Second LLM call failed, returning raw tool output")
			finalAnswer = combined
		} else {
			finalAnswer = finalResp.Content
		}
	}
	return finalAnswer, nil
}

// evaluateAnswer uses the review agent (this method is called on the review agent itself)
// to judge whether the candidate answer is acceptable.
func (a *Agent) evaluateAnswer(ctx context.Context, userMsg string, history []llm.Message, candidate string) (bool, string, error) {
	evalMessages := []llm.Message{
		{Role: "system", Content: a.SystemPrompt},
		{Role: "user", Content: fmt.Sprintf(
			"User question: %s\n\nConversation history:\n%s\n\nCandidate answer: %s\n\nEvaluate whether the candidate answer adequately addresses the user question. Respond with JSON only: {\"acceptable\": true/false, \"feedback\": \"...\" }",
			userMsg,
			formatHistory(history),
			candidate,
		)},
	}

	resp, err := a.llmClient.Chat(ctx, &llm.ChatRequest{
		Messages:  evalMessages,
		MaxTokens: 512,
	})
	if err != nil {
		return false, "", err
	}

	// Extract JSON from response content
	content := strings.TrimSpace(resp.Content)
	if !strings.HasPrefix(content, "{") {
		start := strings.Index(content, "{")
		end := strings.LastIndex(content, "}")
		if start != -1 && end > start {
			content = content[start : end+1]
		}
	}

	var eval struct {
		Acceptable bool   `json:"acceptable"`
		Feedback   string `json:"feedback"`
	}
	if err := json.Unmarshal([]byte(content), &eval); err != nil {
		return false, "", fmt.Errorf("failed to parse review JSON: %w", err)
	}
	return eval.Acceptable, eval.Feedback, nil
}

// getHistoryFromSession retrieves the conversation history from the session.
func (a *Agent) getHistoryFromSession(session *Session) []llm.Message {
	if history, ok := session.Memory["history"]; ok {
		if histSlice, ok := history.([]llm.Message); ok {
			return histSlice
		}
	}
	return nil
}

// formatHistory converts a slice of messages into a readable string for evaluation.
func formatHistory(history []llm.Message) string {
	if len(history) == 0 {
		return "(empty)"
	}
	var b strings.Builder
	for _, m := range history {
		b.WriteString(fmt.Sprintf("%s: %s\n", strings.ToUpper(m.Role), m.Content))
	}
	return b.String()
}

// buildMessages constructs the full message list for the LLM request,
// including the system prompt, conversation history, and the current user message.
func (a *Agent) buildMessages(session *Session, userMsg string) []llm.Message {
	var messages []llm.Message
	messages = append(messages, llm.Message{Role: "system", Content: a.SystemPrompt})

	if history := a.getHistoryFromSession(session); history != nil {
		messages = append(messages, history...)
	}

	messages = append(messages, llm.Message{Role: "user", Content: userMsg})
	return messages
}

// updateHistory appends the user message and agent reply to the session history.
func (a *Agent) updateHistory(session *Session, userMsg, agentReply string) {
	var hist []llm.Message
	if existing, ok := session.Memory["history"]; ok {
		if h, ok := existing.([]llm.Message); ok {
			hist = h
		}
	}
	hist = append(hist, llm.Message{Role: "user", Content: userMsg})
	hist = append(hist, llm.Message{Role: "assistant", Content: agentReply})
	// limit history length to prevent token overflow (keep last 20 exchanges)
	const maxHistory = 20
	if len(hist) > maxHistory {
		hist = hist[len(hist)-maxHistory:]
	}
	session.Memory["history"] = hist
}

// getToolDefs returns the list of tool definitions that this agent is allowed to use.
func (a *Agent) getToolDefs() []llm.ToolDef {
	var defs []llm.ToolDef
	for _, name := range a.Tools {
		tool := a.toolRegistry.Get(name)
		if tool != nil {
			defs = append(defs, tool.Describe())
		} else {
			a.logger.Warn().Str("tool", name).Msg("Tool not found in registry, skipping")
		}
	}
	return defs
}

// MissingTools returns a slice of tool names that are listed in the agent's
// configuration but are not present in the tool registry. This can be used
// during startup to detect configuration errors.
func (a *Agent) MissingTools() []string {
	var missing []string
	for _, name := range a.Tools {
		if a.toolRegistry.Get(name) == nil {
			missing = append(missing, name)
		}
	}
	return missing
}