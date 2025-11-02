package mcphost

import (
	"time"

	"github.com/google/uuid"

	"github.com/d4l-data4life/go-mcp-host/pkg/agent"
	"github.com/d4l-data4life/go-mcp-host/pkg/llm"
)

// ChatRequest represents a chat request to the agent
type ChatRequest struct {
	// ConversationID identifies the conversation
	ConversationID uuid.UUID

	// UserID identifies the user
	UserID uuid.UUID

	// BearerToken is the user's authentication token (optional, used for HTTP MCP servers)
	BearerToken string

	// UserMessage is the user's message text
	UserMessage string

	// Messages is the full message history (optional, if not provided, will be loaded from DB)
	Messages []llm.Message

	// Model is the LLM model to use (optional, defaults to agent config)
	Model string
}

// ChatResponse represents the agent's response
type ChatResponse struct {
	// Message is the assistant's response message
	Message llm.Message

	// ToolsUsed lists all tools that were executed during this chat
	ToolsUsed []ToolExecution

	// Iterations is the number of LLM calls made
	Iterations int

	// TotalTokens is the total number of tokens used
	TotalTokens int

	// Error contains any error that occurred
	Error error
}

// StreamEvent represents a streaming event from the agent
type StreamEvent struct {
	// Type identifies the event type
	Type StreamEventType

	// Content is the text content (for content events)
	Content string

	// Tool is the tool execution info (for tool events)
	Tool *ToolExecution

	// Delta is the streaming delta (for partial content)
	Delta *llm.Delta

	// Done indicates if the stream is complete
	Done bool

	// Error contains any error that occurred
	Error error
}

// StreamEventType defines types of streaming events
type StreamEventType string

const (
	// StreamEventTypeContent indicates text content
	StreamEventTypeContent StreamEventType = "content"

	// StreamEventTypeToolStart indicates a tool execution is starting
	StreamEventTypeToolStart StreamEventType = "tool_start"

	// StreamEventTypeToolComplete indicates a tool execution is complete
	StreamEventTypeToolComplete StreamEventType = "tool_complete"

	// StreamEventTypeDone indicates the stream is complete
	StreamEventTypeDone StreamEventType = "done"

	// StreamEventTypeError indicates an error occurred
	StreamEventTypeError StreamEventType = "error"
)

// ToolExecution represents a tool execution
type ToolExecution struct {
	// ServerName is the MCP server that provided this tool
	ServerName string

	// ToolName is the name of the tool that was called
	ToolName string

	// Arguments are the arguments passed to the tool
	Arguments map[string]interface{}

	// Result is the tool's result (as text)
	Result string

	// Error is any error that occurred during execution
	Error error

	// Duration is how long the tool took to execute
	Duration time.Duration
}

// ToolInfo represents information about an available tool
type ToolInfo = agent.ToolInfo

// ResourceInfo represents information about an available resource
type ResourceInfo = agent.ResourceInfo
