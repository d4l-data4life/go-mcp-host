# Using go-mcp-host as a Library

This guide explains how to use go-mcp-host as a Go library in your own applications.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [API Reference](#api-reference)
- [Configuration](#configuration)
- [Advanced Usage](#advanced-usage)
- [Best Practices](#best-practices)

## Installation

```bash
go get github.com/d4l-data4life/go-mcp-host
```

**Requirements:**
- Go 1.24+
- PostgreSQL database
- Ollama (or OpenAI-compatible LLM endpoint)

## Quick Start

### Minimal Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/d4l-data4life/go-mcp-host/pkg/config"
    "github.com/d4l-data4life/go-mcp-host/pkg/mcphost"
    "github.com/google/uuid"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func main() {
    // 1. Setup database
    db, err := gorm.Open(postgres.Open("your-connection-string"), &gorm.Config{})
    if err != nil {
        log.Fatal(err)
    }

    // 2. Create MCP Host
    host, err := mcphost.NewHost(context.Background(), mcphost.Config{
        MCPServers: []config.MCPServerConfig{
            {
                Name:    "weather",
                Type:    "stdio",
                Command: "npx",
                Args:    []string{"-y", "@h1deya/mcp-server-weather"},
                Enabled: true,
            },
        },
        OpenAIBaseURL: "http://localhost:11434",
        DB:          db,
    })
    if err != nil {
        log.Fatal(err)
    }

    // 3. Use it!
    response, err := host.Chat(context.Background(), mcphost.ChatRequest{
        ConversationID: uuid.New(),
        UserID:         uuid.New(),
        UserMessage:    "What's the weather?",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(response.Message.Content)
}
```

## Core Concepts

### 1. Host

The `Host` is the main entry point. It manages:
- MCP server connections
- LLM client
- Agent orchestration
- Conversation persistence

```go
host, err := mcphost.NewHost(ctx, config)
```

### 2. MCP Servers

MCP servers provide tools and resources. There are two types:

**Stdio servers** (local processes):
```go
{
    Name:    "weather",
    Type:    "stdio",
    Command: "npx",
    Args:    []string{"-y", "@h1deya/mcp-server-weather"},
    Enabled: true,
}
```

**HTTP servers** (remote APIs):
```go
{
    Name:    "my-api",
    Type:    "http",
    URL:     "https://api.example.com/mcp",
    Headers: map[string]string{"Authorization": "Bearer token"},
    ForwardBearer: false,  // Set true to forward user's token
    Enabled: true,
}
```

### 3. Agent

The agent orchestrates the interaction:
1. Receives user message
2. Discovers available MCP tools
3. Sends to LLM with tool descriptions
4. Executes any tool calls
5. Returns final response

This happens automatically when you call `host.Chat()`.

### 4. Conversations

Conversations are persistent and multi-turn:
```go
conversationID := uuid.New()

// Turn 1
host.Chat(ctx, mcphost.ChatRequest{
    ConversationID: conversationID,
    UserMessage:    "Hello",
})

// Turn 2 - maintains context
host.Chat(ctx, mcphost.ChatRequest{
    ConversationID: conversationID,
    UserMessage:    "What did I just say?",
})
```

## API Reference

### Host Methods

#### `NewHost(ctx context.Context, cfg Config) (*Host, error)`

Creates a new MCP Host instance.

**Parameters:**
- `ctx` - Context for initialization
- `cfg` - Configuration (see [Configuration](#configuration))

**Returns:**
- `*Host` - The host instance
- `error` - Any initialization error

---

#### `Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)`

Sends a message and returns the complete response.

**Parameters:**
- `ctx` - Context (for cancellation)
- `req` - Chat request

**Returns:**
- `*ChatResponse` - The agent's response
- `error` - Any error

**Example:**
```go
response, err := host.Chat(ctx, mcphost.ChatRequest{
    ConversationID: conversationID,
    UserID:         userID,
    UserMessage:    "Hello!",
})
```

---

#### `ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)`

Sends a message and returns a streaming response channel.

**Parameters:**
- `ctx` - Context
- `req` - Chat request

**Returns:**
- `<-chan StreamEvent` - Channel of streaming events
- `error` - Any error

**Example:**
```go
stream, err := host.ChatStream(ctx, mcphost.ChatRequest{
    ConversationID: conversationID,
    UserID:         userID,
    UserMessage:    "Tell me a story",
})

for event := range stream {
    switch event.Type {
    case mcphost.StreamEventTypeContent:
        fmt.Print(event.Content)
    case mcphost.StreamEventTypeDone:
        fmt.Println("\nDone!")
    }
}
```

---

#### `GetAvailableTools(ctx context.Context, userID uuid.UUID, bearerToken string) ([]ToolInfo, error)`

Lists all tools available to a specific user, using the short-lived per-user cache.

---

#### `GetAvailableResources(ctx context.Context, userID uuid.UUID, bearerToken string) ([]ResourceInfo, error)`

Lists all resources available to a specific user, using the short-lived per-user cache.

---

#### `CloseConversation(conversationID uuid.UUID) error`

Closes a conversation and cleans up MCP sessions.

Always call this when done with a conversation to free resources.

---

### Types

#### `ChatRequest`

```go
type ChatRequest struct {
    ConversationID uuid.UUID  // Required: Conversation identifier
    UserID         uuid.UUID  // Required: User identifier
    BearerToken    string     // Optional: For HTTP MCP servers
    UserMessage    string     // Required: User's message
    Messages       []Message  // Optional: Override message history
    Model          string     // Optional: Override LLM model
}
```

#### `ChatResponse`

```go
type ChatResponse struct {
    Message     Message         // Assistant's message
    ToolsUsed   []ToolExecution // Tools executed
    Iterations  int             // Number of LLM calls
    TotalTokens int             // Tokens consumed
    Error       error           // Any error
}
```

#### `StreamEvent`

```go
type StreamEvent struct {
    Type    StreamEventType  // Event type
    Content string          // Text content (for content events)
    Tool    *ToolExecution  // Tool info (for tool events)
    Done    bool            // Stream complete
    Error   error           // Any error
}
```

**Event Types:**
- `StreamEventTypeContent` - Text content chunk
- `StreamEventTypeToolStart` - Tool execution started
- `StreamEventTypeToolComplete` - Tool execution finished
- `StreamEventTypeDone` - Stream complete
- `StreamEventTypeError` - Error occurred

## Configuration

### Full Configuration Example

```go
host, err := mcphost.NewHost(ctx, mcphost.Config{
    // MCP Servers to connect to
    MCPServers: []config.MCPServerConfig{
        {
            Name:        "weather",
            Type:        "stdio",
            Command:     "npx",
            Args:        []string{"-y", "@h1deya/mcp-server-weather"},
            Enabled:     true,
            Description: "Weather information",
        },
        {
            Name:          "my-api",
            Type:          "http",
            URL:           "https://api.example.com/mcp",
            Headers:       map[string]string{"X-API-Key": "secret"},
            ForwardBearer: true,  // Forward user's bearer token
            Enabled:       true,
            Description:   "My custom API",
        },
    },

    // LLM endpoint
    OpenAIBaseURL: "http://localhost:11434",

    // Database connection
    DB: db,

    // Agent configuration (optional)
    AgentConfig: agent.Config{
        MaxIterations:        10,
        MaxContextTokens:     8192,
        ToolExecutionTimeout: 60 * time.Second,
        SystemPrompt:         "You are a helpful assistant.",
        DefaultModel:         "llama3.2",
        Temperature:          &temperature,  // 0.7
        MaxTokens:            &maxTokens,    // 4096
        TopP:                 &topP,         // 0.95
    },
})
```

### Configuration Options Explained

**MaxIterations**: Maximum number of tool execution loops (default: 10)
- Prevents infinite loops
- Higher = more complex multi-step reasoning

**MaxContextTokens**: Maximum tokens in conversation context (default: 8192)
- Older messages are truncated if exceeded
- Match to your LLM's context window

**ToolExecutionTimeout**: Timeout for individual tool calls (default: 60s)
- Prevents hanging on slow tools
- Adjust based on your tools

**SystemPrompt**: Instructions for the AI agent
- Customize behavior and personality
- Set constraints and guidelines

**Temperature**: LLM sampling temperature (0.0 - 2.0)
- Lower = more deterministic
- Higher = more creative

## Advanced Usage

### Custom Agent Behavior

```go
temp := 0.3  // More deterministic
maxTokens := 2048

host, _ := mcphost.NewHost(ctx, mcphost.Config{
    // ... servers ...
    AgentConfig: agent.Config{
        SystemPrompt: `You are a weather assistant. 
            Always provide temperatures in Celsius.
            Be concise and factual.`,
        Temperature: &temp,
        MaxTokens: &maxTokens,
    },
})
```

### Multi-User Support

```go
// Each user gets their own conversations
user1ID := uuid.New()
user2ID := uuid.New()

// User 1's conversation
host.Chat(ctx, mcphost.ChatRequest{
    ConversationID: conv1ID,
    UserID:         user1ID,
    UserMessage:    "Hello",
})

// User 2's conversation (isolated)
host.Chat(ctx, mcphost.ChatRequest{
    ConversationID: conv2ID,
    UserID:         user2ID,
    UserMessage:    "Hi",
})
```

### Bearer Token Forwarding

For HTTP MCP servers that require user authentication:

```go
// Configure server to forward bearer tokens
{
    Name:          "authenticated-api",
    Type:          "http",
    URL:           "https://api.example.com/mcp",
    ForwardBearer: true,  // Enable forwarding
    Enabled:       true,
}

// Pass user's token with request
host.Chat(ctx, mcphost.ChatRequest{
    ConversationID: conversationID,
    UserID:         userID,
    BearerToken:    userAuthToken,  // Will be forwarded to MCP server
    UserMessage:    "Get my data",
})
```

### Error Handling

```go
response, err := host.Chat(ctx, request)
if err != nil {
    // Handle initialization errors
    return err
}

if response.Error != nil {
    // Handle agent execution errors
    // (LLM failures, tool errors, etc.)
    log.Printf("Agent error: %v", response.Error)
}

// Check individual tool failures
for _, tool := range response.ToolsUsed {
    if tool.Error != nil {
        log.Printf("Tool %s failed: %v", tool.ToolName, tool.Error)
    }
}
```

### Graceful Shutdown

```go
// When shutting down your application
func shutdown(host *mcphost.Host) {
    // Close all active conversations
    for _, convID := range activeConversations {
        host.CloseConversation(convID)
    }
}
```

## Best Practices

### 1. Reuse Host Instances

```go
// ✅ Good: Create once, reuse
host, _ := mcphost.NewHost(ctx, config)
defer cleanup(host)

for request := range requests {
    host.Chat(ctx, request)
}

// ❌ Bad: Creating new host for each request
for request := range requests {
    host, _ := mcphost.NewHost(ctx, config)  // Expensive!
    host.Chat(ctx, request)
}
```

### 2. Always Close Conversations

```go
conversationID := uuid.New()
defer host.CloseConversation(conversationID)

// Use conversation...
```

### 3. Use Context for Timeouts

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

response, err := host.Chat(ctx, request)
```

### 4. Handle Streaming Properly

```go
stream, err := host.ChatStream(ctx, request)
if err != nil {
    return err
}

// Always drain the channel
for event := range stream {
    if event.Error != nil {
        log.Printf("Stream error: %v", event.Error)
        break
    }
    handleEvent(event)
}
```

### 5. Database Connection Pooling

```go
db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})

// Configure connection pool
sqlDB, _ := db.DB()
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(100)
sqlDB.SetConnMaxLifetime(time.Hour)
```

### 6. Monitoring and Logging

```go
response, err := host.Chat(ctx, request)

// Log useful metrics
log.Printf("Chat request: user=%s conv=%s iterations=%d tokens=%d tools=%d",
    request.UserID,
    request.ConversationID,
    response.Iterations,
    response.TotalTokens,
    len(response.ToolsUsed),
)
```

## Examples

See the [examples/](../examples/) directory for complete, runnable examples:

- `simple_library.go` - Basic library usage
- `embed_in_webserver.go` - Web application integration
- `agent_chat.go` - Advanced agent configuration

## Troubleshooting

### "Failed to initialize MCP client"

- Check that the MCP server command/URL is correct
- For stdio: Ensure Node.js is installed
- For HTTP: Verify the URL is accessible

### "Context deadline exceeded"

- Tool execution took too long
- Increase `ToolExecutionTimeout` in agent config
- Or pass a longer timeout context

### "No tools discovered"

- MCP servers may take time to initialize
- Add a small delay after creating the host
- Check MCP server logs for errors

### Memory Issues

- Close conversations when done
- Limit `MaxIterations` to prevent loops
- Truncate long conversation histories

## Support

- GitHub Issues: [Report bugs](https://github.com/d4l-data4life/go-mcp-host/issues)
- Documentation: [Full docs](https://github.com/d4l-data4life/go-mcp-host/docs)
- Examples: [Code examples](https://github.com/d4l-data4life/go-mcp-host/examples)
