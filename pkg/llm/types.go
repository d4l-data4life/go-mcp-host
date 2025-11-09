package llm

import (
	"context"
)

// Client defines the interface for LLM clients
type Client interface {
	// Chat sends a chat request and returns the complete response
	Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error)

	// ChatStream sends a chat request and returns a channel for streaming responses
	ChatStream(ctx context.Context, request ChatRequest) (<-chan StreamChunk, error)

	// ListModels returns available models
	ListModels(ctx context.Context) ([]Model, error)
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID      string  `json:"id"`
	Model   string  `json:"model"`
	Message Message `json:"message"`
	Usage   Usage   `json:"usage,omitempty"`
}

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	ID    string `json:"id"`
	Model string `json:"model"`
	Delta Delta  `json:"delta"`
	// Message contains the full assistant message when streaming reaches a
	// terminal chunk so that callers no longer need to re-assemble content.
	Message *Message `json:"message,omitempty"`
	Usage   Usage    `json:"usage,omitempty"`
	Done    bool     `json:"done"`
	Error   error    `json:"-"`
}

// Message represents a chat message
type Message struct {
	Role       string     `json:"role"` // system, user, assistant, tool
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// Delta represents incremental content in a stream
type Delta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Tool represents a tool/function that the LLM can call
type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction defines a function tool
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall represents a tool call made by the LLM
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // "function"
	Function ToolCallFunction `json:"function"`
	Index    int              `json:"-"`
}

// ToolCallFunction represents the function part of a tool call
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Model represents an available LLM model
type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Size        int64  `json:"size,omitempty"`
	ModifiedAt  string `json:"modified_at,omitempty"`
}

// Role constants
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// ToolType constants
const (
	ToolTypeFunction = "function"
)
