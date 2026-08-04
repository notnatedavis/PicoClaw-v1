//   pkg/agent/agent.go

//   Package agent provides the core AI agent implementation, including session management,
//   message processing, tool calling, and optional review by a secondary agent

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

// Session holds the conversational state for a single agent interaction
// can be extended to store history, metadata, or other per-session data
type Session struct {
	ID     string                 // unique session identifier
	Memory map[string]interface{} // simple key-value store for ephemeral memory
}

// AgentConfig carries the initialisation parameters for creating an Agent
type AgentConfig struct {
	ID           string   // unique agent identifier
	Name         string   // human-readable name
	Model        string   // LLM model name (e.g., "llama3.2:3b")
	SystemPrompt string   // system prompt that defines the agent's persona
	Tools        []string // list of tool names this agent is allowed to use
	Workspace    string   // working directory for file operations
}

// Agent is a self-contained AI agent with its own configuration, LLM client,
// tool registry, and pipeline. processes user messages and may invoke tools
type Agent struct {
	ID           string
	Name         string
	Model        string
	SystemPrompt string
	Tools        []string
	Workspace    string

	llmClient    llm.Client      // client for interacting with the LLM provider
	toolRegistry *tools.Registry // registry of available tools
	pipeline     *LLMPipeline    // pipeline that orchestrates LLM calls and tool execution
	logger       zerolog.Logger  // structured logger with agent context
	registry     *Registry       // reference to the global agent registry (for cross-agent calls)
}

// NewAgent creates and fully initialises a new Agent instance
// It constructs the LLM pipeline and sets up the logger with the agent ID
func NewAgent(cfg AgentConfig, llmClient llm.Client, toolReg *tools.Registry, logger zerolog.Logger, agentReg *Registry) *Agent {
	pipeline := NewLLMPipeline(llmClient, toolReg, logger)
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

// process is the public entry point for processing a user message
// executes a single turn and returns the final answer
func (a *Agent) Process(ctx context.Context, userMsg string, session *Session) (string, error) {
	return a.runTurn(ctx, userMsg, session)
}

// runTurn handles the complete processing of one user message :
//  1. build the message list (system + user + history from session)
//  2. retrieve the tool definitions allowed for this agent
//  3. call the LLM pipeline to get a response (which may include tool calls)
//  4. if a tool call is requested, execute the tool and return its result
//  5. optionally, pass the answer through a review agent for validation
//  6. return the final answer
func (a *Agent) runTurn(ctx context.Context, userMsg string, session *Session) (string, error) {
	a.logger.Info().Msg("Processing turn")

	// 1: build the message list for the LLM
	messages := a.buildMessages(session, userMsg)
	toolDefs := a.getToolDefs()

	// 2: call the LLM pipeline
	resp, err := a.pipeline.Run(ctx, messages, toolDefs)
	if err != nil {
		return "", fmt.Errorf("LLM pipeline error: %w", err)
	}

	// 3: handle any tool calls requested by the model
	if len(resp.ToolCalls) > 0 {
		tc := resp.ToolCalls[0] // only one tool call per turn (simplified)
		tool := a.toolRegistry.Get(tc.Name)
		if tool == nil {
			return "", fmt.Errorf("tool %s not found", tc.Name)
		}
		result, err := tool.Execute(ctx, tc.Parameters)
		if err != nil {
			return "", fmt.Errorf("tool execution failed: %w", err)
		}
		final := fmt.Sprintf("%v", result)
		a.logger.Info().Str("result", final).Msg("Tool executed successfully")
		return final, nil
	}

	// 4: extract the plain-text answer (if no tool call)
	answer := resp.Content
	if answer == "" {
		answer = "I'm sorry, I didn't understand that."
	}

	// 5: (Optional) validate/refine the answer via a review agent
	if a.registry != nil {
		reviewAgent := a.registry.Get("review")
		if reviewAgent != nil {
			reviewed, err := a.callReviewAgent(ctx, reviewAgent, userMsg, answer)
			if err != nil {
				a.logger.Warn().Err(err).Msg("Review agent failed, using original answer")
			} else {
				answer = reviewed
			}
		}
	}

	return answer, nil
}

// callReviewAgent invokes a separate "review" agent with the original query
// and the candidate answer. review agent can correct or refine the answer,
// or even execute additional tools if needed
func (a *Agent) callReviewAgent(ctx context.Context, review *Agent, query, candidate string) (string, error) {
	// create a fresh session for the review agent.
	reviewSession := &Session{
		ID:     fmt.Sprintf("review-%s", time.Now().Format("20060102150405")),
		Memory: map[string]interface{}{},
	}
	// build a prompt instructing the reviewer to validate and improve the answer.
	prompt := fmt.Sprintf(
		"Original query: %s\n\nAssistant's raw answer: %s\n\nPlease validate and, if needed, refine the answer. If the answer is a tool call (JSON), execute it and return the actual result. Otherwise, ensure the answer is correct and polite. Return only the final answer, nothing else.",
		query, candidate,
	)
	reviewed, err := review.runTurn(ctx, prompt, reviewSession)
	if err != nil {
		return candidate, err
	}
	return reviewed, nil
}

// buildMessages constructs the full message list for the LLM request
// includes the system prompt and the current user message
// (In a full implementation, it would also include historical messages from session.Memory.)
func (a *Agent) buildMessages(session *Session, userMsg string) []llm.Message {
	messages := []llm.Message{
		{Role: "system", Content: a.SystemPrompt},
	}

	// TODO: add previous conversation history from session.Memory if needed.
	
	messages = append(messages, llm.Message{Role: "user", Content: userMsg})
	return messages
}

// getToolDefs returns the list of tool definitions that this agent is allowed to use
// filters the global tool registry by the agent's configured tool names
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