//   pkg/gateway/gateway.go

//   Package gateway provides the HTTP entry point and orchestrates agent loading,
//   message routing, and starting the server

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/picoclaw/pkg/agent"
	"github.com/picoclaw/pkg/llm"
	"github.com/picoclaw/pkg/tools"
	"github.com/rs/zerolog"
)

// Config represents the top-level configuration structure from config/config.json
type Config struct {
	Gateway struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		LogLevel string `json:"log_level"`
	} `json:"gateway"`
	Agents struct {
		Defaults struct {
			Workspace           string `json:"workspace"`
			Provider            string `json:"provider"`
			ModelName           string `json:"model_name"`
			MaxTokens           int    `json:"max_tokens"`
			MaxToolIterations   int    `json:"max_tool_iterations"`
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
	Tools struct {
		Web           ToolsEnabled `json:"web"`
		Exec          ToolsEnabled `json:"exec"`
		Filesystem    ToolsEnabled `json:"filesystem"`
		Spawn         ToolsEnabled `json:"spawn"`
		Subagent      ToolsEnabled `json:"subagent"`
		Cron          ToolsEnabled `json:"cron"`
		Message       ToolsEnabled `json:"message"`
		ReadFile      ToolsEnabled `json:"read_file"`
		WriteFile     ToolsEnabled `json:"write_file"`
		ListDir       ToolsEnabled `json:"list_dir"`
		EditFile      ToolsEnabled `json:"edit_file"`
		AppendFile    ToolsEnabled `json:"append_file"`
		WebFetch      ToolsEnabled `json:"web_fetch"`
		SendFile      ToolsEnabled `json:"send_file"`
		SendTTS       ToolsEnabled `json:"send_tts"`
		LoadImage     ToolsEnabled `json:"load_image"`
		FindSkills    ToolsEnabled `json:"find_skills"`
		InstallSkill  ToolsEnabled `json:"install_skill"`
		Skills        ToolsEnabled `json:"skills"`
		MCP           ToolsEnabled `json:"mcp"`
		MediaCleanup  ToolsEnabled `json:"media_cleanup"`
		I2C           ToolsEnabled `json:"i2c"`
		Serial        ToolsEnabled `json:"serial"`
		SPI           ToolsEnabled `json:"spi"`
	} `json:"tools"`
}

// ToolsEnabled holds the enabled flag for a tool section
type ToolsEnabled struct {
	Enabled bool `json:"enabled"`
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
	sessions      map[string]*agent.Session // session store keyed by session ID
	sessionsMu    sync.Mutex
}

// NewGateway reads the configuration file at configPath and constructs a Gateway instance
func NewGateway(configPath string, logger zerolog.Logger) (*Gateway, error) {
	// Read and parse config file
	data, err := os.ReadFile(configPath)
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
		sessions:      make(map[string]*agent.Session),
	}, nil
}

// InitAgents loads all agent definitions from the filesystem, builds the tool registry,
// creates each agent with its configuration, and sets up the LLM client.
func (g *Gateway) InitAgents() error {
	// 1: Build the global tool registry and register all configured tools
	toolReg := tools.NewRegistry()
	g.registerConfiguredTools(toolReg)

	// 2: Instantiate the LLM client from the first model_list entry
	if len(g.config.ModelList) == 0 {
		return fmt.Errorf("no model_list entry found in config")
	}
	modelCfg := g.config.ModelList[0]
	client, err := llm.NewOpenAIClient(modelCfg.APIBase, modelCfg.ModelName, g.logger)
	if err != nil {
		return fmt.Errorf("failed to create LLM client: %w", err)
	}
	g.llmClient = client
	g.logger.Info().Str("model", modelCfg.ModelName).Str("base", modelCfg.APIBase).Msg("LLM client initialised")

	// 3: Determine model name, max tokens, and workspace from defaults
	model := g.config.Agents.Defaults.ModelName
	if model == "" {
		model = modelCfg.ModelName
	}
	maxTokens := g.config.Agents.Defaults.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	baseWorkspace := g.config.Agents.Defaults.Workspace
	if baseWorkspace == "" {
		baseWorkspace = "./workspace"
	}

	// 4: Load each agent from the list
	for _, entry := range g.config.Agents.List {
		agentFilePath := filepath.Join("config", "agents", entry.Name+".json")
		data, err := os.ReadFile(agentFilePath)
		if err != nil {
			return fmt.Errorf("could not read agent config %s: %w", agentFilePath, err)
		}
		var afc AgentFileConfig
		if err := json.Unmarshal(data, &afc); err != nil {
			return fmt.Errorf("could not parse agent config %s: %w", agentFilePath, err)
		}

		// enforce lowercase agent ID (case‑insensitive routing)
		agentID := strings.ToLower(afc.Name)

		// build the agent configuration
		agentCfg := agent.AgentConfig{
			ID:           agentID,
			Name:         afc.Name,
			Model:        model,
			SystemPrompt: afc.SystemPrompt,
			Tools:        afc.Tools,
			Workspace:    filepath.Join(baseWorkspace, "agent-sessions", agentID),
			MaxTokens:    maxTokens,
		}

		// create the agent with the real LLM client
		a := agent.NewAgent(agentCfg, g.llmClient, toolReg, g.logger, g.agentRegistry)
		g.agentRegistry.Register(a)
		g.logger.Info().Str("agent", agentID).Msg("Agent registered")
	}
	return nil
}

// registerConfiguredTools adds stub implementations for every tool that is enabled in the configuration.
// This prevents "tool not found" errors when the LLM requests a tool.
func (g *Gateway) registerConfiguredTools(toolReg *tools.Registry) {
	// A minimal stub that satisfies tools.Tool
	type stubTool struct {
		name        string
		description string
		params      map[string]interface{}
	}
	execStub := func(name string, params map[string]interface{}) (interface{}, error) {
		return fmt.Sprintf("[%s stub] called with params: %v", name, params), nil
	}

	register := func(enabled bool, name, desc string, params map[string]interface{}) {
		if enabled {
			toolReg.Register(name, &stubTool{name: name, description: desc, params: params})
			g.logger.Debug().Str("tool", name).Msg("Registered tool")
		}
	}

	t := g.config.Tools
	register(t.Web.Enabled, "web", "Web search stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"query": map[string]interface{}{"type": "string"}}})
	register(t.Exec.Enabled, "exec", "Shell command execution stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"command": map[string]interface{}{"type": "string"}}})
	register(t.Filesystem.Enabled, "filesystem", "Filesystem operations stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]interface{}{"type": "string"}, "operation": map[string]interface{}{"type": "string"}}})
	register(t.Spawn.Enabled, "spawn", "Subprocess spawning stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"cmd": map[string]interface{}{"type": "string"}}})
	register(t.Subagent.Enabled, "subagent", "Sub‑agent invocation stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"agent": map[string]interface{}{"type": "string"}, "task": map[string]interface{}{"type": "string"}}})
	register(t.Cron.Enabled, "cron", "Cron job stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"schedule": map[string]interface{}{"type": "string"}, "command": map[string]interface{}{"type": "string"}}})
	register(t.Message.Enabled, "message", "Messaging stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"target": map[string]interface{}{"type": "string"}, "text": map[string]interface{}{"type": "string"}}})
	register(t.ReadFile.Enabled, "read_file", "Read file stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]interface{}{"type": "string"}}})
	register(t.WriteFile.Enabled, "write_file", "Write file stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]interface{}{"type": "string"}, "content": map[string]interface{}{"type": "string"}}})
	register(t.ListDir.Enabled, "list_dir", "List directory stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]interface{}{"type": "string"}}})
	register(t.EditFile.Enabled, "edit_file", "Edit file stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]interface{}{"type": "string"}, "changes": map[string]interface{}{"type": "string"}}})
	register(t.AppendFile.Enabled, "append_file", "Append to file stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]interface{}{"type": "string"}, "content": map[string]interface{}{"type": "string"}}})
	register(t.WebFetch.Enabled, "web_fetch", "Fetch URL stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"url": map[string]interface{}{"type": "string"}}})
	register(t.SendFile.Enabled, "send_file", "Send file stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]interface{}{"type": "string"}}})
	register(t.SendTTS.Enabled, "send_tts", "TTS stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"text": map[string]interface{}{"type": "string"}}})
	register(t.LoadImage.Enabled, "load_image", "Load image stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]interface{}{"type": "string"}}})
	register(t.FindSkills.Enabled, "find_skills", "Find skills stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"query": map[string]interface{}{"type": "string"}}})
	register(t.InstallSkill.Enabled, "install_skill", "Install skill stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"skill": map[string]interface{}{"type": "string"}}})
	register(t.Skills.Enabled, "skills", "Skills management stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"action": map[string]interface{}{"type": "string"}}})
	register(t.MCP.Enabled, "mcp", "MCP stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"name": map[string]interface{}{"type": "string"}}})
	register(t.MediaCleanup.Enabled, "media_cleanup", "Media cleanup stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]interface{}{"type": "string"}}})
	register(t.I2C.Enabled, "i2c", "I2C stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"address": map[string]interface{}{"type": "string"}}})
	register(t.Serial.Enabled, "serial", "Serial port stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"port": map[string]interface{}{"type": "string"}}})
	register(t.SPI.Enabled, "spi", "SPI stub", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"device": map[string]interface{}{"type": "string"}}})
}

// stubTool implements tools.Tool
func (s *stubTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return fmt.Sprintf("[%s stub] called with params: %v", s.name, params), nil
}
func (s *stubTool) Describe() llm.ToolDef {
	return llm.ToolDef{
		Name:        s.name,
		Description: s.description,
		Parameters:  s.params,
	}
}

// ProcessMessage routes a user message to the specified agent and returns the agent's reply.
// It accepts a session ID to maintain conversation history across calls.
func (g *Gateway) ProcessMessage(agentID, sessionID, userMessage string) (string, error) {
	agentID = strings.ToLower(agentID) // case‑insensitive lookup
	a := g.agentRegistry.Get(agentID)
	if a == nil {
		return "", fmt.Errorf("agent %s not found", agentID)
	}

	// obtain or create a session
	g.sessionsMu.Lock()
	sess, exists := g.sessions[sessionID]
	if !exists {
		sess = &agent.Session{
			ID:     sessionID,
			Memory: make(map[string]interface{}),
		}
		g.sessions[sessionID] = sess
	}
	g.sessionsMu.Unlock()

	// process message
	return a.Process(context.Background(), userMessage, sess)
}

// Start launches the HTTP server and registers the /chat endpoint.
func (g *Gateway) Start() error {
	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		agentID := r.URL.Query().Get("agent")
		if agentID == "" {
			agentID = "default"
		}
		sessionID := r.URL.Query().Get("session")
		if sessionID == "" {
			sessionID = "default"
		}
		msg := r.URL.Query().Get("msg")
		if msg == "" {
			http.Error(w, "missing msg parameter", http.StatusBadRequest)
			return
		}
		reply, err := g.ProcessMessage(agentID, sessionID, msg)
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