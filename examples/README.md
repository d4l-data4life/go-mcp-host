# go-mcp-host Examples

This directory contains examples demonstrating how to use go-mcp-host both as a library and as a service.

## Prerequisites

Before running these examples, ensure you have:

1. **Go 1.24+** installed
2. **PostgreSQL** running (or use the provided Docker command)
3. **Ollama** running locally with a model installed
4. **Node.js** (for MCP servers via npx)

### Quick Setup

```bash
# 1. Start PostgreSQL
make docker-database

# 2. Install and start Ollama
# Download from https://ollama.ai or:
curl -fsSL https://ollama.ai/install.sh | sh
ollama serve

# 3. Pull a model
ollama pull llama3.2

# 4. Verify everything is running
curl http://localhost:11434/api/tags  # Should list your models
psql -h localhost -p 6000 -U go-mcp-host -d go-mcp-host  # Should connect
```

## Examples

### 1. simple_library.go - Simplest Library Usage

**Use case:** You want the easiest way to add MCP Host capabilities to your application.

This example uses the high-level `mcphost` package API, which is the recommended way to use go-mcp-host as a library.

**Run:**
```bash
go run examples/simple_library/simple_library.go
```

**Key features demonstrated:**
- Simple initialization with minimal configuration
- Synchronous chat requests
- Streaming chat responses
- Listing available tools
- Proper cleanup

**Best for:** Quick integration, prototyping, simple use cases

---

### 2. embed_in_webserver.go - Web Application Integration

**Use case:** You have an existing web application and want to add AI agent capabilities.

This example shows how to embed go-mcp-host into your own HTTP server, exposing it via REST API endpoints.

**Run:**
```bash
go run examples/embed_in_webserver/embed_in_webserver.go
```

Then open http://localhost:8080 in your browser to interact with the demo UI, or use curl:

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "What'\''s the weather in Paris?"}'
```

**Key features demonstrated:**
- Embedding MCP Host in a chi router
- REST API endpoints for chat
- Server-Sent Events (SSE) for streaming
- Simple web UI for interaction
- Integration with existing application routes

**Best for:** Existing web applications, microservices, REST APIs

---

### 3. agent_chat - Using the Agent Package

**Use case:** You need more control over the agent behavior and want to use the mid-level API.

This example uses the `agent` package directly, giving you more control over configuration and behavior.

**Run:**
```bash
go run examples/agent_chat/agent_chat.go
```

**Key features demonstrated:**
- Direct use of the Agent package
- Custom agent configuration (temperature, max tokens, etc.)
- Both streaming and non-streaming modes
- Detailed statistics (iterations, tokens, tool executions)
- Manual conversation management

**Best for:** Advanced use cases, custom agent behavior, full control

---

### 4. ollama_with_mcp - Low-Level MCP Integration

**Use case:** You want maximum control or need to understand the internals.

This example shows the lowest-level usage, manually orchestrating the MCP manager, LLM client, and tool execution loop.

**Run:**
```bash
go run examples/ollama_with_mcp/ollama_with_mcp.go
```

**Key features demonstrated:**
- Manual MCP session creation
- Direct MCP client usage
- Tool discovery and conversion to LLM format
- Manual tool execution loop
- Understanding the internal flow

**Best for:** Learning internals, custom orchestration, debugging

---

## Configuration

### Using Different MCP Servers

All examples can be modified to use different MCP servers. Here are some popular ones:

```go
// Weather server
{
    Name:    "weather",
    Type:    "stdio",
    Command: "npx",
    Args:    []string{"-y", "@h1deya/mcp-server-weather"},
    Enabled: true,
}

// Filesystem server
{
    Name:    "filesystem",
    Type:    "stdio",
    Command: "npx",
    Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir"},
    Enabled: true,
}

// PostgreSQL server
{
    Name:    "postgres",
    Type:    "stdio",
    Command: "npx",
    Args:    []string{"-y", "@modelcontextprotocol/server-postgres", "postgresql://..."},
    Enabled: true,
}

// HTTP server (custom)
{
    Name:    "my-api",
    Type:    "http",
    URL:     "https://api.example.com/mcp",
    Headers: map[string]string{"Authorization": "Bearer token"},
    Enabled: true,
}
```

### Environment Variables

You can customize behavior with environment variables:

```bash
# Database
export DATABASE_URL="host=localhost port=5432 user=myuser password=mypass dbname=mydb sslmode=disable"

# LLM
export OLLAMA_URL="http://localhost:11434"
export LLM_MODEL="llama3.2"

# Port
export PORT="9090"
```

## Common Patterns

### Pattern 1: Single Request

```go
response, err := host.Chat(ctx, mcphost.ChatRequest{
    ConversationID: conversationID,
    UserID:         userID,
    UserMessage:    "Hello!",
})
```

### Pattern 2: Streaming with UI Updates

```go
stream, err := host.ChatStream(ctx, mcphost.ChatRequest{...})
for event := range stream {
    switch event.Type {
    case mcphost.StreamEventTypeContent:
        updateUI(event.Content)  // Update your UI
    case mcphost.StreamEventTypeToolStart:
        showToolIndicator(event.Tool.ToolName)
    }
}
```

### Pattern 3: Multi-Turn Conversation

```go
// Turn 1
response1, _ := host.Chat(ctx, mcphost.ChatRequest{
    ConversationID: conversationID,
    UserID:         userID,
    UserMessage:    "What's the weather in Tokyo?",
})

// Turn 2 - uses same conversation ID to maintain context
response2, _ := host.Chat(ctx, mcphost.ChatRequest{
    ConversationID: conversationID,  // Same ID
    UserID:         userID,
    UserMessage:    "How about New York?",
})
```

### Pattern 4: Custom System Prompt

```go
host, _ := mcphost.NewHost(ctx, mcphost.Config{
    // ... other config ...
    AgentConfig: agent.Config{
        SystemPrompt: "You are a helpful assistant specializing in weather data. Always provide temperature in Celsius.",
    },
})
```

## Troubleshooting

### "Failed to connect to database"

Make sure PostgreSQL is running:
```bash
make docker-database
# Or check your DATABASE_URL
```

### "Failed to connect to Ollama"

Make sure Ollama is running and has a model:
```bash
ollama serve          # Start Ollama
ollama pull llama3.2  # Pull a model
```

### "Failed to initialize MCP client"

Make sure Node.js is installed and the MCP server package is available:
```bash
node --version  # Should be v18+
npx -y @h1deya/mcp-server-weather  # Test manually
```

### "No tools discovered"

Some MCP servers take a moment to initialize. Add a small delay:
```go
time.Sleep(2 * time.Second)
```

## Next Steps

After trying these examples:

1. **Read the documentation**: Check [docs/MCP_IMPLEMENTATION.md](../docs/MCP_IMPLEMENTATION.md)
2. **Explore the API**: See [pkg/mcphost/](../pkg/mcphost/) for the public API
3. **Deploy it**: Check [deploy/README.md](../deploy/README.md) for deployment options
4. **Customize it**: Modify the examples for your specific use case

## Contributing

Found a bug or have an improvement? Please open an issue or PR on GitHub!

## Resources

- [Model Context Protocol Docs](https://modelcontextprotocol.io/)
- [Ollama Documentation](https://github.com/ollama/ollama/blob/main/docs/api.md)
- [Available MCP Servers](https://github.com/modelcontextprotocol)

