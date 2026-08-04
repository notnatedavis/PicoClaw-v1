//   pkg/gateway/gateway.go

//   Package gateway provides the HTTP entry point and orchestrates agent loading,
//   message routing, and starting the server

package gateway

import (
	"context"
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

// Config represents the top-level configuration structure from config/config.json
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

// AgentFileConfig mirrors the JSON schema of each agent file in config/agents/
type AgentFileConfig struct {
	Name         string   `json:"name"`
	SystemPrompt string   `json:"system_prompt"`
	Tools        []string `json:"tools"`
	Memory       bool     `json:"memory"`
}

// Gateway is the main application coordinator. It loads the configuration,
// initialises all agents and tools, and runs an HTTP server to accept chat requests
type Gateway struct {
	config        Config
	agentRegistry *agent.Registry
	logger        zerolog.Logger
	llmClient     llm.Client // set during initialisation (e.g., Ollama client)
}

// NewGateway reads the configuration file at configPath and constructs a Gateway instance
func NewGateway(configPath string, logger zerolog.Logger) (*Gateway, error) {
	// Read and parse config file
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

// InitAgents loads all agent definitions from the filesystem, builds the tool registry,
// and creates each agent with its configuration. also sets up the LLM client (currently nil,
// but would be instantiated based on the provider in a real implementation)
func (g *Gateway) InitAgents() error {
	// 1: Build the global tool registry
	toolReg := tools.NewRegistry()
	// Placeholder: register example tools (in production, all tools from config would be registered)
	toolReg.Register("echo", &echoTool{})
	toolReg.Register("filesystem", &fsTool{})

	// TODO: Register all tools listed in config.json

	// 2: determine the default model name from the config
	model := g.config.Agents.Defaults.ModelName

	// 3: iterate over each agent entry and load its JSON file
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

		// build the agent configuration
		agentCfg := agent.AgentConfig{
			ID:           afc.Name,
			Name:         afc.Name,
			Model:        model,
			SystemPrompt: afc.SystemPrompt,
			Tools:        afc.Tools,
			Workspace:    filepath.Join("workspace", "agent-sessions", afc.Name),
		}

		// create the agent (llmClient is nil for now; it would be set later).
		a := agent.NewAgent(agentCfg, nil, toolReg, g.logger, g.agentRegistry)
		g.agentRegistry.Register(a)
		g.logger.Info().Str("agent", afc.Name).Msg("Agent registered")
	}
	return nil
}

// ProcessMessage routes a user message to the specified agent and returns the agent's reply
// It creates a fresh session for each call (no session persistence)
func (g *Gateway) ProcessMessage(agentID, userMessage string) (string, error) {
	a := g.agentRegistry.Get(agentID)
	if a == nil {
		return "", fmt.Errorf("agent %s not found", agentID)
	}
	// Create a new session for this request.
	session := &agent.Session{
		ID:     fmt.Sprintf("session-%d", os.Getpid()),
		Memory: make(map[string]interface{}),
	}
	// Call the agent's Process method.
	return a.Process(context.Background(), userMessage, session)
}

// Start launches the HTTP server and registers the /chat endpoint
// It listens on the host and port specified in the config
func (g *Gateway) Start() error {
	// define the /chat handler
	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		// extract parameters
		agentID := r.URL.Query().Get("agent")
		if agentID == "" {
			agentID = "default"
		}
		msg := r.URL.Query().Get("msg")
		if msg == "" {
			http.Error(w, "missing msg parameter", http.StatusBadRequest)
			return
		}
		// process the message
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

// ---- Example tool implementations (placeholders) ----

// echoTool simply echoes back the input text.
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

// fsTool is a minimal placeholder for filesystem operations
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