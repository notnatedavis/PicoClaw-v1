//   pkg/agent/agent.go

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

// Session holds the conversation state for an agent
type Session struct {
	ID     string
	Memory map[string]interface{} // simple key-value memory, extend as needed
}

// AgentConfig carries initialisation params for agent
type AgentConfig struct {
	ID           string
	Name         string
	Model        string
	SystemPrompt string
	Tools        []string // tool names this agent is allowed to use
	Workspace    string
}

// Agent is self-contained AI agent
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
	registry     *Registry // for accessing other agents
}

// NewAgent creates + fully initialises an Agent
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

// runTurn executes a single turn with the user message.
func (a *Agent) runTurn(ctx context.Context, userMsg string, session *Session) (string, error) {
	a.logger.Info().Msg("Processing turn")

	messages := a.buildMessages(session, userMsg)
	toolDefs := a.getToolDefs()

	resp, err := a.pipeline.Run(ctx, messages, toolDefs)
	if err != nil {
		return "", fmt.Errorf("LLM pipeline error: %w", err)
	}

	// handle tool calls if any
	if len(resp.ToolCalls) > 0 {
		tc := resp.ToolCalls[0]
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

	answer := resp.Content
	if answer == "" {
		answer = "I'm sorry, I didn't understand that."
	}

	// ---- REVIEW STEP ----
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

// callReviewAgent invokes the review agent with the original query and candidate answer.
func (a *Agent) callReviewAgent(ctx context.Context, review *Agent, query, candidate string) (string, error) {
	reviewSession := &Session{
		ID:     fmt.Sprintf("review-%s", time.Now().Format("20060102150405")),
		Memory: map[string]interface{}{},
	}
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

// buildMessages creates the message list from the session memory and user message.
func (a *Agent) buildMessages(session *Session, userMsg string) []llm.Message {
	messages := []llm.Message{
		{Role: "system", Content: a.SystemPrompt},
	}
	// In a full implementation, session.Memory would hold previous turns.
	// For simplicity we only add the current user message.
	messages = append(messages, llm.Message{Role: "user", Content: userMsg})
	return messages
}

// getToolDefs returns the list of tool definitions that this agent may use.
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