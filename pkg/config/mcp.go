package config

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// MCPServerConfig represents configuration for an MCP server connection
type MCPServerConfig struct {
	Name          string            `yaml:"name"                  json:"name"`
	Type          string            `yaml:"type"                  json:"type"` // "stdio" or "http"
	Command       string            `yaml:"command,omitempty"     json:"command,omitempty"`
	Args          []string          `yaml:"args,omitempty"        json:"args,omitempty"`
	Env           map[string]string `yaml:"env,omitempty"         json:"env,omitempty"`
	URL           string            `yaml:"url,omitempty"         json:"url,omitempty"`
	Headers       map[string]string `yaml:"headers,omitempty"     json:"headers,omitempty"`
	ForwardBearer bool              `yaml:"forwardBearer"         json:"forwardBearer"` // When true, the current user's bearer token will be forwarded as Authorization header.
	Enabled       bool              `yaml:"enabled"               json:"enabled"`
	Description   string            `yaml:"description,omitempty" json:"description,omitempty"`
}

// OllamaConfig represents configuration for Ollama LLM
type OllamaConfig struct {
	BaseURL        string  `yaml:"baseUrl"        json:"baseUrl"`
	DefaultModel   string  `yaml:"defaultModel"   json:"defaultModel"`
	Temperature    float64 `yaml:"temperature"    json:"temperature"`
	MaxTokens      int     `yaml:"maxTokens"      json:"maxTokens"`
	TopP           float64 `yaml:"topP"           json:"topP"`
	RequestTimeout string  `yaml:"requestTimeout" json:"requestTimeout"`
}

// MCPConfig represents configuration for MCP-related settings
type MCPConfig struct {
	Servers            []MCPServerConfig `yaml:"servers"            json:"servers"`
	SessionTimeout     string            `yaml:"sessionTimeout"     json:"sessionTimeout"`
	MaxSessionsPerUser int               `yaml:"maxSessionsPerUser" json:"maxSessionsPerUser"`
	ReconnectAttempts  int               `yaml:"reconnectAttempts"  json:"reconnectAttempts"`
	ReconnectDelay     string            `yaml:"reconnectDelay"     json:"reconnectDelay"`
}

// AgentConfig represents configuration for the agent orchestrator
type AgentConfig struct {
	MaxIterations        int    `yaml:"maxIterations"        json:"maxIterations"`
	MaxContextTokens     int    `yaml:"maxContextTokens"     json:"maxContextTokens"`
	ToolExecutionTimeout string `yaml:"toolExecutionTimeout" json:"toolExecutionTimeout"`
	DefaultModel         string `yaml:"defaultModel"         json:"defaultModel"`
}

// GetMCPConfig returns MCP configuration from viper
func GetMCPConfig() MCPConfig {
	return MCPConfig{
		Servers:            GetMCPServers(),
		SessionTimeout:     viper.GetString("MCP_SESSION_TIMEOUT"),
		MaxSessionsPerUser: viper.GetInt("MCP_MAX_SESSIONS_PER_USER"),
		ReconnectAttempts:  viper.GetInt("MCP_RECONNECT_ATTEMPTS"),
		ReconnectDelay:     viper.GetString("MCP_RECONNECT_DELAY"),
	}
}

// GetMCPServers returns configured MCP servers from viper
func GetMCPServers() []MCPServerConfig {
	var servers []MCPServerConfig
	if err := viper.UnmarshalKey("mcp_servers", &servers); err != nil {
		// Log the error and return default configuration
		fmt.Printf("Warning: Failed to unmarshal mcp_servers from config: %v\n", err)
		return []MCPServerConfig{
			{
				Name:        "filesystem",
				Type:        "stdio",
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
				Enabled:     false,
				Description: "Local filesystem access",
			},
		}
	}
	fmt.Printf("Loaded %d MCP servers from configuration\n", len(servers))
	for i, s := range servers {
		fmt.Printf("  [%d] %s (%s) - enabled: %v\n", i+1, s.Name, s.Type, s.Enabled)
	}
	return servers
}

// GetOllamaConfig returns Ollama configuration from viper
func GetOllamaConfig() OllamaConfig {
	return OllamaConfig{
		BaseURL:        viper.GetString("OLLAMA_BASE_URL"),
		DefaultModel:   viper.GetString("OLLAMA_DEFAULT_MODEL"),
		Temperature:    viper.GetFloat64("OLLAMA_TEMPERATURE"),
		MaxTokens:      viper.GetInt("OLLAMA_MAX_TOKENS"),
		TopP:           viper.GetFloat64("OLLAMA_TOP_P"),
		RequestTimeout: viper.GetString("OLLAMA_REQUEST_TIMEOUT"),
	}
}

// GetAgentConfig returns agent configuration from viper
func GetAgentConfig() AgentConfig {
	return AgentConfig{
		MaxIterations:        viper.GetInt("AGENT_MAX_ITERATIONS"),
		MaxContextTokens:     viper.GetInt("AGENT_MAX_CONTEXT_TOKENS"),
		ToolExecutionTimeout: viper.GetString("AGENT_TOOL_EXECUTION_TIMEOUT"),
		DefaultModel:         viper.GetString("OLLAMA_DEFAULT_MODEL"),
	}
}

// SetupMCPEnv configures MCP-related environment variables
func SetupMCPEnv() {
	// Ollama configuration
	bindEnvVariable("OLLAMA_BASE_URL", "http://localhost:11434")
	bindEnvVariable("OLLAMA_DEFAULT_MODEL", "llama3.2")
	bindEnvVariable("OLLAMA_TEMPERATURE", 0.7)
	bindEnvVariable("OLLAMA_MAX_TOKENS", 4096)
	bindEnvVariable("OLLAMA_TOP_P", 0.9)
	bindEnvVariable("OLLAMA_REQUEST_TIMEOUT", "300s")

	// MCP configuration
	bindEnvVariable("MCP_SESSION_TIMEOUT", "1h")
	bindEnvVariable("MCP_MAX_SESSIONS_PER_USER", 10)
	bindEnvVariable("MCP_RECONNECT_ATTEMPTS", 3)
	bindEnvVariable("MCP_RECONNECT_DELAY", "5s")

	// Agent configuration
	bindEnvVariable("AGENT_MAX_ITERATIONS", 10)
	bindEnvVariable("AGENT_MAX_CONTEXT_TOKENS", 8192)
	bindEnvVariable("AGENT_TOOL_EXECUTION_TIMEOUT", "60s")

	// MCP Servers configuration (can be overridden via config file)
	// Example servers are commented out by default
	// Users should configure in config.yaml or environment
}

// FullMCPConfig combines all MCP-related configurations
type FullMCPConfig struct {
	Servers []MCPServerConfig
	Ollama  OllamaConfig
	Agent   AgentConfig
}

// LoadMCPConfig loads all MCP-related configuration
func LoadMCPConfig() (*FullMCPConfig, error) {
	// Try to load from config file
	if err := viper.ReadInConfig(); err != nil {
		// Config file not required, just use environment variables
		logging.LogDebugf("No config.yaml found, using environment variables and defaults")
	}

	cfg := &FullMCPConfig{
		Servers: GetMCPServers(),
		Ollama:  GetOllamaConfig(),
		Agent:   GetAgentConfig(),
	}

	return cfg, nil
}
