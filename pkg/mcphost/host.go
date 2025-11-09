package mcphost

import (
	"context"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/d4l-data4life/go-mcp-host/pkg/agent"
	"github.com/d4l-data4life/go-mcp-host/pkg/config"
	"github.com/d4l-data4life/go-mcp-host/pkg/llm"
	llmopenai "github.com/d4l-data4life/go-mcp-host/pkg/llm/openai"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/manager"
)

// Host is the main entry point for embedding MCP Host functionality in your application.
// It provides a simple API for creating AI agents with access to multiple MCP servers.
type Host struct {
	agent      *agent.Agent
	mcpManager *manager.Manager
	llmClient  llm.Client
	config     Config
}

// Config configures the MCP Host
type Config struct {
	// MCPServers is the list of MCP servers to connect to
	MCPServers []config.MCPServerConfig

	// OpenAIBaseURL is the base URL of your OpenAI-compatible endpoint.
	// Example: "https://api.openai.com/v1" or "http://localhost:11434/v1" for Ollama.
	OpenAIBaseURL string

	// OpenAIAPIKey optionally overrides the API key for the OpenAI-compatible endpoint.
	OpenAIAPIKey string

	// OpenAIModel is the default model to use for the LLM client.
	OpenAIDefaultModel string

	// LLMClient allows providing a fully custom llm.Client implementation.
	LLMClient llm.Client

	// Database connection (required for conversation persistence)
	DB *gorm.DB

	// Agent configuration (optional, defaults will be used)
	AgentConfig agent.Config
}

// NewHost creates a new MCP Host instance.
//
// Example:
//
//	host, err := mcphost.NewHost(ctx, mcphost.Config{
//	    MCPServers: []config.MCPServerConfig{
//	        {
//	            Name:    "weather",
//	            Type:    "stdio",
//	            Command: "npx",
//	            Args:    []string{"-y", "@h1deya/mcp-server-weather"},
//	            Enabled: true,
//	        },
//	    },
//	    OpenAIBaseURL: "http://localhost:11434",
//	    DB:          db,
//	})
func NewHost(ctx context.Context, cfg Config) (*Host, error) {
	// Create MCP manager
	mcpManager := manager.NewMCPManager(cfg.MCPServers)
	openAIConfig := config.GetOpenAIConfig()

	// Set agent defaults if not provided
	agentConfig := cfg.AgentConfig

	// Create LLM client
	llmClient := cfg.LLMClient
	if llmClient == nil {
		baseURL := cfg.OpenAIBaseURL
		if baseURL == "" {
			baseURL = openAIConfig.BaseURL
		}
		apiKey := cfg.OpenAIAPIKey
		if apiKey == "" {
			apiKey = openAIConfig.APIKey
		}
		if agentConfig.DefaultModel == "" {
			agentConfig.DefaultModel = cfg.OpenAIDefaultModel
		}
		llmClient = llmopenai.NewClient(llmopenai.Config{
			APIKey:  apiKey,
			BaseURL: baseURL,
			Model:   agentConfig.DefaultModel,
		})
	}

	// Create agent
	agent := agent.NewAgent(cfg.DB, mcpManager, llmClient, agentConfig)

	return &Host{
		agent:      agent,
		mcpManager: mcpManager,
		llmClient:  llmClient,
		config:     cfg,
	}, nil
}

// Chat sends a message to the agent and returns the response.
// This is a blocking call that waits for the full response.
//
// Example:
//
//	response, err := host.Chat(ctx, mcphost.ChatRequest{
//	    ConversationID: conversationID,
//	    UserID:         userID,
//	    UserMessage:    "What's the weather in New York?",
//	})
func (h *Host) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	agentReq := agent.ChatRequest{
		ConversationID: req.ConversationID,
		UserID:         req.UserID,
		BearerToken:    req.BearerToken,
		UserMessage:    req.UserMessage,
		Messages:       req.Messages,
		Model:          req.Model,
	}

	agentResp, err := h.agent.Chat(ctx, agentReq)
	if err != nil {
		return nil, err
	}

	return &ChatResponse{
		Message:     agentResp.Message,
		ToolsUsed:   convertToolExecutions(agentResp.ToolsUsed),
		Iterations:  agentResp.Iterations,
		TotalTokens: agentResp.TotalTokens,
		Error:       agentResp.Error,
	}, nil
}

// ChatStream sends a message to the agent and returns a streaming response channel.
// Use this for real-time streaming of the agent's response.
//
// Example:
//
//	eventChan, err := host.ChatStream(ctx, mcphost.ChatRequest{
//	    ConversationID: conversationID,
//	    UserID:         userID,
//	    UserMessage:    "Tell me a story",
//	})
//	for event := range eventChan {
//	    if event.Error != nil {
//	        log.Printf("Error: %v", event.Error)
//	        break
//	    }
//	    fmt.Print(event.Content)
//	}
func (h *Host) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	agentReq := agent.ChatRequest{
		ConversationID: req.ConversationID,
		UserID:         req.UserID,
		BearerToken:    req.BearerToken,
		UserMessage:    req.UserMessage,
		Messages:       req.Messages,
		Model:          req.Model,
	}

	agentStream, err := h.agent.ChatStream(ctx, agentReq)
	if err != nil {
		return nil, err
	}

	// Convert agent stream to host stream
	hostStream := make(chan StreamEvent, 10)
	go func() {
		defer close(hostStream)
		for event := range agentStream {
			hostStream <- StreamEvent{
				Type:    StreamEventType(event.Type),
				Content: event.Content,
				Tool:    convertToolExecution(event.Tool),
				Delta:   event.Delta,
				Done:    event.Done,
				Error:   event.Error,
			}
		}
	}()

	return hostStream, nil
}

// GetAvailableTools returns all available tools for a conversation.
func (h *Host) GetAvailableTools(ctx context.Context, conversationID uuid.UUID) ([]ToolInfo, error) {
	return h.agent.GetAvailableTools(ctx, conversationID)
}

// GetAvailableResources returns all available resources for a conversation.
func (h *Host) GetAvailableResources(ctx context.Context, conversationID uuid.UUID) ([]ResourceInfo, error) {
	return h.agent.GetAvailableResources(ctx, conversationID)
}

// CloseConversation cleans up resources for a conversation.
// Call this when a conversation is complete to free up MCP sessions.
func (h *Host) CloseConversation(conversationID uuid.UUID) error {
	return h.agent.CloseConversation(conversationID)
}

// ServeHTTP sets up HTTP routes on the provided chi router.
// This is useful if you want to expose the MCP Host as a web service.
//
// Example:
//
//	r := chi.NewRouter()
//	host.ServeHTTP(r)
//	http.ListenAndServe(":8080", r)
func (h *Host) ServeHTTP(r chi.Router) {
	// This would integrate with the existing handlers package
	// For now, this is a placeholder - full implementation would wire up
	// handlers with the host's agent and mcpManager
	// TODO: Implement route setup
}

// Agent returns the underlying agent for advanced usage
func (h *Host) Agent() *agent.Agent {
	return h.agent
}

// MCPManager returns the underlying MCP manager for advanced usage
func (h *Host) MCPManager() *manager.Manager {
	return h.mcpManager
}

// LLMClient returns the underlying LLM client for advanced usage
func (h *Host) LLMClient() llm.Client {
	return h.llmClient
}

// Helper conversion functions

func convertToolExecutions(executions []agent.ToolExecution) []ToolExecution {
	result := make([]ToolExecution, len(executions))
	for i, e := range executions {
		result[i] = ToolExecution{
			ServerName: e.ServerName,
			ToolName:   e.ToolName,
			Arguments:  e.Arguments,
			Result:     e.Result,
			Error:      e.Error,
			Duration:   e.Duration,
		}
	}
	return result
}

func convertToolExecution(execution *agent.ToolExecution) *ToolExecution {
	if execution == nil {
		return nil
	}
	return &ToolExecution{
		ServerName: execution.ServerName,
		ToolName:   execution.ToolName,
		Arguments:  execution.Arguments,
		Result:     execution.Result,
		Error:      execution.Error,
		Duration:   execution.Duration,
	}
}
