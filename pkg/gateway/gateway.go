// pkg/gateway/gateway.go
package gateway

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/picoclaw/pkg/agent"
	"github.com/picoclaw/pkg/llm"
	"github.com/picoclaw/pkg/tools"
	"github.com/rs/zerolog"
)

// Config reflects the structure of config/config.json.
type Config struct {
	Gateway struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"gateway"`
	Agents struct {
		Defaults struct {
			Provider  string `json:"provider"`
			ModelName string `json:"model_name"`
			MaxTokens int    `json:"max_tokens"`
		} `json:"defaults"`
		List []struct {
			Name string `json:"name"`
		} `json:"list"`
	} `json:"agents"`
	ModelList []struct {
		ModelName string `json:"model_name"`
		Provider  string `json:"provider"`
		APIBase   string `json:"api_base"`
	} `json:"model_list"`
}

// AgentFileConfig mirrors the per‑agent JSON file in config/agents/.
type AgentFileConfig struct {
	Name         string   `json:"name"`
	SystemPrompt string   `json:"system_prompt"`
	Tools        []string `json:"tools"`
	Memory       bool     `json:"memory"`
}

// Gateway is the main application coordinator.
type Gateway struct {
	config        Config
	agentRegistry *agent.Registry
	logger        zerolog.Logger
	llmClient     llm.Client // will be initialised later (e.g., Ollama client)
}

// NewGateway creates a new Gateway with the given config path.
func NewGateway(configPath string, logger zerolog.Logger) (*Gateway, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("could not read config file %s: %w", configPath, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse config: %w", err)
	}
	return &Gateway{
		config:        cfg,
		agentRegistry: agent.NewRegistry(),
		logger:        logger,
	}, nil
}

// InitAgents loads all agent definitions, creates tool registries, and initialises agents.
func (g *Gateway) InitAgents() error {
	// 1. Build a global tool registry with all required tools.
	//    In a real setup, tools would be instantiated with their own dependencies.
	toolReg := tools.NewRegistry()
	// Placeholder: register example tools (web, filesystem, exec, etc.)
	// For brevity we add a simple echo tool; a full implementation would add all declared tools.
	toolReg.Register("echo", &echoTool{})
	toolReg.Register("filesystem", &fsTool{}) // minimal example
	// You would register all tools listed in config.json here.

	// 2. Determine model defaults from config.
	model := g.config.Agents.Defaults.ModelName

	// 3. Load each agent from its JSON file in config/agents/.
	for _, entry := range g.config.Agents.List {
		agentFilePath := filepath.Join("config", "agents", entry.Name+".json")
		data, err := ioutil.ReadFile(agentFilePath)
		if err != nil {
			return fmt.Errorf("could not read agent config %s: %w", agentFilePath, err)
		}
		var afc AgentFileConfig
		if err := json.Unmarshal(data, &afc); err != nil {
			return fmt.Errorf("could not parse agent config %s: %w", agentFilePath, err)
		}

		agentCfg := agent.AgentConfig{
			ID:           afc.Name, // agent ID = name for simplicity
			Name:         afc.Name,
			Model:        model,
			SystemPrompt: afc.SystemPrompt,
			Tools:        afc.Tools,
			Workspace:    filepath.Join("workspace", "agent-sessions", afc.Name),
		}

		// For now we use a nil llm.Client – the actual client would be set after initialising the LLM provider.
		// In production, you'd create an OllamaClient that implements llm.Client.
		a := agent.NewAgent(agentCfg, nil, toolReg, g.logger, g.agentRegistry)
		g.agentRegistry.Register(a)
		g.logger.Info().Str("agent", afc.Name).Msg("Agent registered")
	}
	return nil
}

// ProcessMessage sends a message to an agent (by ID) and returns the response.
func (g *Gateway) ProcessMessage(agentID, userMessage string) (string, error) {
	a := g.agentRegistry.Get(agentID)
	if a == nil {
		return "", fmt.Errorf("agent %s not found", agentID)
	}
	// Each call gets a fresh session for now.
	session := &agent.Session{
		ID:     fmt.Sprintf("session-%d", os.Getpid()),
		Memory: make(map[string]interface{}),
	}
	// Note: runTurn is unexported; we can add a public method Process in agent if needed.
	// For internal testing, we can call a similar exported function; we'll expose RunTurn.
	// So we'll add a wrapper: a.Process(ctx, userMsg, session)
	// Let's define a public method on Agent:
	// func (a *Agent) Process(ctx context.Context, msg string, session *Session) (string, error) {
	//     return a.runTurn(ctx, msg, session)
	// }
	// We'll assume it's exported.
	return a.Process(context.Background(), userMessage, session)
}

// Start launches a simple HTTP server to test the gateway.
func (g *Gateway) Start() error {
	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		agentID := r.URL.Query().Get("agent")
		if agentID == "" {
			agentID = "default"
		}
		msg := r.URL.Query().Get("msg")
		if msg == "" {
			http.Error(w, "missing msg parameter", http.StatusBadRequest)
			return
		}
		reply, err := g.ProcessMessage(agentID, msg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, reply)
	})

	addr := fmt.Sprintf("%s:%d", g.config.Gateway.Host, g.config.Gateway.Port)
	g.logger.Info().Str("addr", addr).Msg("Gateway HTTP server starting")
	return http.ListenAndServe(addr, nil)
}

// ---- Example tool implementations (placeholder) ----

type echoTool struct{}

func (e *echoTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	text, _ := params["text"].(string)
	return "echo: " + text, nil
}

func (e *echoTool) Describe() llm.ToolDef {
	return llm.ToolDef{
		Name:        "echo",
		Description: "Echoes back the input text.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{"type": "string"},
			},
		},
	}
}

type fsTool struct{}

func (f *fsTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path, _ := params["path"].(string)
	return "fs operation on " + path + " (not implemented)", nil
}

func (f *fsTool) Describe() llm.ToolDef {
	return llm.ToolDef{
		Name:        "filesystem",
		Description: "Performs filesystem operations.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string"},
			},
		},
	}
}

// Ensure Agent has a public Process method (add this to agent.go)
// (We'll note it's needed – already added in the final agent.go above