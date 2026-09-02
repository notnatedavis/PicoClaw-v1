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
			Review              struct {
				Enabled       bool   `json:"enabled"`
				MaxIterations int    `json:"max_iterations"`
				Agent         string `json:"agent"`
			} `json:"review"`
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

		// Newly implemented tools
		DateTime     ToolsEnabled `json:"date_time"`
		Memory       ToolsEnabled `json:"memory"`
		WebSearch    ToolsEnabled `json:"web_search"`
		HTTPRequest  ToolsEnabled `json:"http_request"`
		FileSearch   ToolsEnabled `json:"file_search"`
		Weather      ToolsEnabled `json:"weather"`
		Calculator   ToolsEnabled `json:"calculator"`
		SystemInfo   ToolsEnabled `json:"system_info"`
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
	Internal     bool     `json:"internal"` // if true, agent is not routable via /chat
}

// Gateway is the main application coordinator. It loads the configuration,
// initialises all agents and tools, and runs an HTTP server to accept chat requests
type Gateway struct {
	config        Config
	agentRegistry *agent.Registry
	logger        zerolog.Logger
	llmClient     llm.Client                    // set during initialisation (e.g., Ollama client)
	sessions      map[string]*agent.Session     // session store keyed by session ID
	sessionsMu    sync.Mutex
	reviewAgent   *agent.Agent                  // internal review agent (may be nil)
}

// NewGateway reads the configuration file at configPath and constructs a Gateway instance
func NewGateway(configPath string, logger zerolog.Logger) (*Gateway, error) {
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
// If review is enabled, it also loads the review agent and injects it into all main agents.
func (g *Gateway) InitAgents() error {
	// 1: Build the global tool registry and register all configured tools
	toolReg := tools.NewRegistry()

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

	// Register tools (real implementations and stubs) with the base workspace
	g.registerConfiguredTools(toolReg, baseWorkspace)

	// 4: Load each main agent from the list
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

		// Add new tools to the agent's tool list if they are enabled globally
		// and not already explicitly listed in the agent config.
		if g.config.Tools.DateTime.Enabled {
			afc.Tools = appendIfMissing(afc.Tools, "date_time")
		}
		if g.config.Tools.Memory.Enabled {
			afc.Tools = appendIfMissing(afc.Tools, "memory")
		}
		if g.config.Tools.WebSearch.Enabled {
			afc.Tools = appendIfMissing(afc.Tools, "web_search")
		}
		if g.config.Tools.HTTPRequest.Enabled {
			afc.Tools = appendIfMissing(afc.Tools, "http_request")
		}
		if g.config.Tools.FileSearch.Enabled {
			afc.Tools = appendIfMissing(afc.Tools, "file_search")
		}
		if g.config.Tools.Weather.Enabled {
			afc.Tools = appendIfMissing(afc.Tools, "weather")
		}
		if g.config.Tools.Calculator.Enabled {
			afc.Tools = appendIfMissing(afc.Tools, "calculator")
		}
		if g.config.Tools.SystemInfo.Enabled {
			afc.Tools = appendIfMissing(afc.Tools, "system_info")
		}

		// Log the agent's full tool list for verification
		g.logger.Info().
			Str("agent", agentID).
			Strs("tools", afc.Tools).
			Msg("Agent tool list")

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

		a := agent.NewAgent(agentCfg, g.llmClient, toolReg, g.logger, g.agentRegistry)
		g.agentRegistry.Register(a)
		g.logger.Info().Str("agent", agentID).Msg("Agent registered")
	}

	// 5: Load review agent if enabled
	if g.config.Agents.Defaults.Review.Enabled {
		reviewName := g.config.Agents.Defaults.Review.Agent
		if reviewName == "" {
			reviewName = "reviewer"
		}
		agentFilePath := filepath.Join("config", "agents", reviewName+".json")
		data, err := os.ReadFile(agentFilePath)
		if err != nil {
			return fmt.Errorf("could not read review agent config %s: %w", agentFilePath, err)
		}
		var afc AgentFileConfig
		if err := json.Unmarshal(data, &afc); err != nil {
			return fmt.Errorf("could not parse review agent config %s: %w", agentFilePath, err)
		}

		reviewID := strings.ToLower(afc.Name)
		reviewAgentCfg := agent.AgentConfig{
			ID:           reviewID,
			Name:         afc.Name,
			Model:        model,
			SystemPrompt: afc.SystemPrompt,
			Tools:        nil, // review agent should not use tools
			Workspace:    filepath.Join(baseWorkspace, "agent-sessions", reviewID),
			MaxTokens:    maxTokens,
		}
		g.reviewAgent = agent.NewAgent(reviewAgentCfg, g.llmClient, toolReg, g.logger, g.agentRegistry)

		maxIter := g.config.Agents.Defaults.Review.MaxIterations
		if maxIter <= 0 {
			maxIter = 3 // default
		}
		// Inject review agent and max iterations into all main agents
		for _, entry := range g.config.Agents.List {
			agentID := strings.ToLower(entry.Name)
			if a := g.agentRegistry.Get(agentID); a != nil {
				a.SetReviewAgent(g.reviewAgent)
				a.SetMaxReviewIterations(maxIter)
			}
		}
		g.logger.Info().Str("review_agent", reviewID).Int("max_iterations", maxIter).Msg("Review agent enabled")
	}

	// 6: Validate that every agent's tools are present in the tool registry
	g.logger.Info().Msg("Validating agent tool availability...")
	for _, entry := range g.config.Agents.List {
		agentID := strings.ToLower(entry.Name)
		a := g.agentRegistry.Get(agentID)
		if a == nil {
			continue
		}
		if missing := a.MissingTools(); len(missing) > 0 {
			g.logger.Error().
				Str("agent", agentID).
				Strs("missing_tools", missing).
				Msg("Agent has tools not present in registry")
		} else {
			g.logger.Info().
				Str("agent", agentID).
				Msg("All configured tools are available")
		}
	}

	return nil
}

// registerConfiguredTools registers real implementations for the newly added tools
// and stub implementations for all other enabled tools that do not yet have a real
// counterpart. This prevents "tool not found" errors when the LLM requests a tool.
// The baseWorkspace parameter is used by tools that need filesystem access (memory, file_search).
func (g *Gateway) registerConfiguredTools(toolReg *tools.Registry, baseWorkspace string) {
	t := g.config.Tools

	// Real implementations for the newly implemented tools
	if t.DateTime.Enabled {
		toolReg.Register("date_time", &tools.DateTimeTool{})
	}
	if t.Memory.Enabled {
		memoryFile := filepath.Join(baseWorkspace, "memory", "global.json")
		toolReg.Register("memory", tools.NewMemoryTool(memoryFile))
	}
	if t.WebSearch.Enabled {
		toolReg.Register("web_search", &tools.WebSearchTool{})
	}
	if t.HTTPRequest.Enabled {
		toolReg.Register("http_request", &tools.HTTPRequestTool{})
	}
	if t.FileSearch.Enabled {
		toolReg.Register("file_search", tools.NewFileSearchTool(baseWorkspace))
	}
	if t.Weather.Enabled {
		toolReg.Register("weather", &tools.WeatherTool{})
	}
	if t.Calculator.Enabled {
		toolReg.Register("calculator", &tools.CalculatorTool{})
	}
	if t.SystemInfo.Enabled {
		toolReg.Register("system_info", &tools.SystemInfoTool{})
	}

	// Stub tools for other enabled tools (not yet fully implemented)
	stubTools := []struct {
		enabled bool
		name    string
	}{
		{t.Web.Enabled, "web"},
		{t.Exec.Enabled, "exec"},
		{t.Filesystem.Enabled, "filesystem"},
		{t.Spawn.Enabled, "spawn"},
		{t.Subagent.Enabled, "subagent"},
		{t.Cron.Enabled, "cron"},
		{t.Message.Enabled, "message"},
		{t.ReadFile.Enabled, "read_file"},
		{t.WriteFile.Enabled, "write_file"},
		{t.ListDir.Enabled, "list_dir"},
		{t.EditFile.Enabled, "edit_file"},
		{t.AppendFile.Enabled, "append_file"},
		{t.WebFetch.Enabled, "web_fetch"},
		{t.SendFile.Enabled, "send_file"},
		{t.SendTTS.Enabled, "send_tts"},
		{t.LoadImage.Enabled, "load_image"},
		{t.FindSkills.Enabled, "find_skills"},
		{t.InstallSkill.Enabled, "install_skill"},
		{t.Skills.Enabled, "skills"},
		{t.MCP.Enabled, "mcp"},
		{t.MediaCleanup.Enabled, "media_cleanup"},
		{t.I2C.Enabled, "i2c"},
		{t.Serial.Enabled, "serial"},
		{t.SPI.Enabled, "spi"},
	}
	for _, s := range stubTools {
		if s.enabled {
			toolReg.Register(s.name, &stubTool{name: s.name})
			g.logger.Debug().Str("tool", s.name).Msg("Registered stub tool")
		}
	}

	// Log the final list of all registered tools
	g.logger.Info().Strs("registered_tools", toolReg.List()).Msg("Global tool registry initialised")
}

// stubTool is a placeholder tool that returns a message indicating it is not yet implemented.
// It satisfies the tools.Tool interface and exists solely to prevent errors when the LLM
// requests a tool that has not been fully implemented.
type stubTool struct {
	name string
}

func (s *stubTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return fmt.Sprintf("Tool '%s' is not implemented yet.", s.name), nil
}

func (s *stubTool) Describe() llm.ToolDef {
	return llm.ToolDef{
		Name:        s.name,
		Description: fmt.Sprintf("Stub for %s (not implemented).", s.name),
		Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	}
}

// appendIfMissing adds an item to a string slice only if it is not already present.
func appendIfMissing(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
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