package agent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"github.com/d4l-data4life/go-mcp-host/pkg/llm"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/manager"
	schemautil "github.com/d4l-data4life/go-mcp-host/pkg/mcp/schemautil"
)

// Agent represents an AI agent that can use MCP tools via LLM
type Agent struct {
	db           *gorm.DB
	mcpManager   *manager.Manager
	llmClient    llm.Client
	orchestrator *Orchestrator
	config       Config
}

// Config holds agent configuration
type Config struct {
	// Maximum number of tool execution iterations
	MaxIterations int

	// Maximum tokens in context
	MaxContextTokens int

	// Tool execution timeout
	ToolExecutionTimeout time.Duration

	// System prompt for the agent
	SystemPrompt string

	// Default LLM model
	DefaultModel string

	// LLM parameters
	Temperature *float64
	MaxTokens   *int
	TopP        *float64
}

// NewAgent creates a new agent instance
func NewAgent(db *gorm.DB, mcpManager *manager.Manager, llmClient llm.Client, cfg Config) *Agent {
	// Set defaults
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 50
	}
	if cfg.MaxContextTokens == 0 {
		cfg.MaxContextTokens = 8192
	}
	if cfg.ToolExecutionTimeout == 0 {
		cfg.ToolExecutionTimeout = 60 * time.Second
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = "You are a helpful AI assistant with access to various tools. Use them to answer user questions accurately."
	}
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = viper.GetString("OPENAI_DEFAULT_MODEL")
	}

	agent := &Agent{
		db:         db,
		mcpManager: mcpManager,
		llmClient:  llmClient,
		config:     cfg,
	}

	// Create orchestrator
	agent.orchestrator = NewOrchestrator(mcpManager, llmClient, cfg)

	return agent
}

// Chat sends a message and returns the agent's response
func (a *Agent) Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error) {
	// Execute orchestration loop (sessions created on-demand)
	response, err := a.orchestrator.Execute(ctx, request)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// ChatStream sends a message and returns a streaming response channel
func (a *Agent) ChatStream(ctx context.Context, request ChatRequest) (<-chan StreamEvent, error) {
	// Execute streaming orchestration (sessions created on-demand)
	return a.orchestrator.ExecuteStream(ctx, request)
}

// CloseConversation cleans up resources for a conversation
func (a *Agent) CloseConversation(conversationID uuid.UUID) error {
	return a.mcpManager.CloseAllSessionsForConversation(conversationID)
}

// GetAvailableTools returns all available tools for a user (short-lived clients + cache)
func (a *Agent) GetAvailableTools(ctx context.Context, userID uuid.UUID, bearerToken string) ([]ToolInfo, error) {
	toolsWithServer, err := a.mcpManager.ListAllToolsForUser(ctx, userID, bearerToken)
	if err != nil {
		return nil, err
	}

	tools := make([]ToolInfo, len(toolsWithServer))
	for i, t := range toolsWithServer {
		tools[i] = ToolInfo{
			ServerName:  t.ServerName,
			ToolName:    t.Tool.Name,
			Description: t.Tool.Description,
			InputSchema: schemautil.ToolSchemaJSON(t.Tool),
		}
	}

	return tools, nil
}

// GetAvailableResources returns all available resources for a user (short-lived clients + cache)
func (a *Agent) GetAvailableResources(ctx context.Context, userID uuid.UUID, bearerToken string) ([]ResourceInfo, error) {
	resourcesWithServer, err := a.mcpManager.ListAllResourcesForUser(ctx, userID, bearerToken)
	if err != nil {
		return nil, err
	}

	resources := make([]ResourceInfo, len(resourcesWithServer))
	for i, r := range resourcesWithServer {
		resources[i] = ResourceInfo{
			ServerName:  r.ServerName,
			URI:         r.Resource.URI,
			Name:        r.Resource.Name,
			Description: r.Resource.Description,
			MimeType:    r.Resource.MIMEType,
		}
	}

	return resources, nil
}

// ChatRequest represents a chat request to the agent
type ChatRequest struct {
	ConversationID uuid.UUID
	UserID         uuid.UUID
	BearerToken    string
	UserMessage    string
	Messages       []llm.Message // Optional: provide full message history
	Model          string        // Optional: override default model
}

// ChatResponse represents the agent's response
type ChatResponse struct {
	Message     llm.Message
	ToolsUsed   []ToolExecution
	Iterations  int
	TotalTokens int
	Error       error
}

// StreamEvent represents a streaming event from the agent
type StreamEvent struct {
	Type    StreamEventType
	Content string
	Tool    *ToolExecution
	Delta   *llm.Delta
	Done    bool
	Error   error
}

// StreamEventType defines types of streaming events
type StreamEventType string

const (
	StreamEventTypeContent      StreamEventType = "content"
	StreamEventTypeToolStart    StreamEventType = "tool_start"
	StreamEventTypeToolComplete StreamEventType = "tool_complete"
	StreamEventTypeDone         StreamEventType = "done"
	StreamEventTypeError        StreamEventType = "error"
)

// ToolExecution represents a tool execution
type ToolExecution struct {
	ServerName string
	ToolName   string
	Arguments  map[string]interface{}
	Result     string
	Error      error
	Duration   time.Duration
}

// ToolInfo represents information about an available tool
type ToolInfo struct {
	ServerName  string
	ToolName    string
	Description string
	InputSchema json.RawMessage
}

// ResourceInfo represents information about an available resource
type ResourceInfo struct {
	ServerName  string
	URI         string
	Name        string
	Description string
	MimeType    string
}
