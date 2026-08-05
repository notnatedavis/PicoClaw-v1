//   pkg/agent/agent.go

//   Package agent provides the core AI agent implementation, including session management,
//   message processing, tool calling, and conversational history.

package agent

import (
	"context"
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

	llmClient    llm.Client
	toolRegistry *tools.Registry
	pipeline     *LLMPipeline
	logger       zerolog.Logger
	registry     *Registry // reference to the global agent registry (for future cross-agent calls)
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

// Process is the public entry point for processing a user message within a session.
func (a *Agent) Process(ctx context.Context, userMsg string, session *Session) (string, error) {
	return a.runTurn(ctx, userMsg, session)
}

// runTurn handles the complete processing of one user message :
//  1. build the message list (system + history + user)
//  2. retrieve the tool definitions allowed for this agent
//  3. call the LLM pipeline to get a response (which may include tool calls)
//  4. if tool calls are requested, execute all tools, feed results back to the LLM,
//     and obtain a final natural-language answer
//  5. update session history with the exchange
//  6. return the final answer
func (a *Agent) runTurn(ctx context.Context, userMsg string, session *Session) (string, error) {
	a.logger.Info().Msg("Processing turn")

	// 1: build messages including conversation history
	messages := a.buildMessages(session, userMsg)
	toolDefs := a.getToolDefs()

	// 2: first LLM call
	resp, err := a.pipeline.Run(ctx, messages, toolDefs)
	if err != nil {
		return "", fmt.Errorf("LLM pipeline error: %w", err)
	}

	// 3: handle tool calls if any
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

		// Second LLM call to produce the final answer
		finalResp, err := a.pipeline.Run(ctx, messages, toolDefs)
		if err != nil {
			// fallback: return raw tool results as answer
			a.logger.Warn().Err(err).Msg("Second LLM call failed, returning raw tool output")
			finalAnswer = combined
		} else {
			finalAnswer = finalResp.Content
		}
	}

	// 4: update session history
	a.updateHistory(session, userMsg, finalAnswer)

	return finalAnswer, nil
}

// buildMessages constructs the full message list for the LLM request,
// including the system prompt, conversation history, and the current user message.
func (a *Agent) buildMessages(session *Session, userMsg string) []llm.Message {
	var messages []llm.Message
	messages = append(messages, llm.Message{Role: "system", Content: a.SystemPrompt})

	// retrieve history from session memory
	if history, ok := session.Memory["history"]; ok {
		if histSlice, ok := history.([]llm.Message); ok {
			messages = append(messages, histSlice...)
		}
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