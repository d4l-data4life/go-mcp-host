# MCP Implementation Guide

This document explains how the MCP (Model Context Protocol) implementation works in go-mcp-host.

## Architecture Overview

The MCP implementation is organized into three main layers:

```
┌─────────────────────────────────────────────┐
│         Manager (pkg/mcp/manager)           │
│  - Manages multiple MCP sessions            │
│  - Per-conversation session lifecycle       │
│  - Tool/Resource aggregation                │
│  - Database persistence                     │
└──────────────────┬──────────────────────────┘
                   │
         ┌─────────┴─────────┐
         │                   │
┌────────▼────────┐  ┌───────▼────────┐
│  Client Layer   │  │  Client Layer  │
│ (mark3labs/     │  │ (per server)   │
│  mcp-go/client) │  │ - Resource read│
│ - Tool calls    │  │ - Prompts      │
│ - Notifications │  │ - Streaming    │
└────────┬────────┘  └───────┬────────┘
         │                   │
┌────────▼────────┐  ┌───────▼────────┐
│ Stdio Transport │  │ HTTP Transport │
│ (local servers) │  │(remote servers)│
└─────────────────┘  └────────────────┘
```

## Components

### 1. Protocol Layer (`github.com/mark3labs/mcp-go/mcp`)

Instead of maintaining our own protocol definitions, we now import the `mcp` package from [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go). It contains the JSON-RPC primitives, typed MCP messages, and helpers (e.g., `mcp.Tool`, `mcp.Resource`, `mcp.InitializeRequest`, `mcp.ClientCapabilities`). This guarantees we stay aligned with the latest MCP specification without duplicating work.

### 2. Transport Layer (`github.com/mark3labs/mcp-go/client/transport`)

The upstream module also provides production-ready transports. We rely on:

- `transport.NewStdioWithOptions` for local processes launched via stdio.
- `transport.NewStreamableHTTP` for HTTP/SSE servers (including bearer/OAuth helpers).

```go
// Stdio example
trans := transport.NewStdioWithOptions(
    "npx",
    []string{"DEBUG=true"},
    []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
)

// HTTP example
trans, err := transport.NewStreamableHTTP(
    "https://api.example.com/mcp",
    transport.WithHTTPHeaders(map[string]string{"Authorization": "Bearer token"}),
    transport.WithContinuousListening(),
)
```

### 3. Client Layer (`github.com/mark3labs/mcp-go/client`)

`mcp-go/client` replaces our previous in-repo client. It handles initialization, paging, notifications, sampling callbacks, etc. Example:

```go
trans := transport.NewStdioWithOptions("npx", nil, []string{"-y", "@modelcontextprotocol/server-filesystem"})
cli := mcpclient.NewClient(trans, mcpclient.WithClientCapabilities(mcp.ClientCapabilities{
    Roots:    &struct{ ListChanged bool }{ListChanged: true},
    Sampling: &struct{}{},
}))

if err := cli.Start(ctx); err != nil {
    log.Fatal(err)
}

if _, err := cli.Initialize(ctx, mcp.InitializeRequest{
    Params: mcp.InitializeParams{
        ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
        ClientInfo:      mcp.Implementation{Name: "go-mcp-host", Version: "1.0.0"},
        Capabilities:    cli.GetClientCapabilities(),
    },
}); err != nil {
    log.Fatal(err)
}

tools, _ := cli.ListTools(ctx, mcp.ListToolsRequest{})
result, _ := cli.CallTool(ctx, mcp.CallToolRequest{
    Params: mcp.CallToolParams{
        Name: "read_file",
        Arguments: map[string]any{
            "path": "/tmp/test.txt",
        },
    },
})
```

All low-level concerns (JSON-RPC framing, SSE parsing, notification fan-out) now live in this upstream package.

### 4. Manager Layer (`pkg/mcp/manager/`)

Manages multiple MCP clients per conversation with lifecycle and caching.

**File:**
- `manager.go` - Session manager implementation

**Manager Usage:**
```go
// Create manager
manager := manager.NewMCPManager(db, 1*time.Hour) // 1 hour session timeout

// Get or create session for conversation (bearer token optional)
session, err := manager.GetOrCreateSession(ctx, conversationID, serverConfig, bearerToken, userID)

// Get all tools available to the user (cached per server)
tools, err := manager.ListAllToolsForUser(ctx, userID, bearerToken)

// Call a tool on a specific server
result, err := manager.CallTool(ctx, conversationID, serverName, toolName, args)

// Get all resources available to the user (cached per server)
resources, err := manager.ListAllResourcesForUser(ctx, userID, bearerToken)

// Read a resource
content, err := manager.ReadResource(ctx, conversationID, serverName, resourceURI)

// Cleanup when conversation ends
err := manager.CloseAllSessionsForConversation(conversationID)
```

**Manager Features:**
- **Session Pooling**: One MCP client per server per conversation
- **Automatic Initialization**: Creates and initializes clients on-demand
- **Tool Caching**: Caches tools in database for fast access
- **Resource Caching**: Caches resource metadata in database
- **Notification Handling**: Automatically refreshes caches when servers notify changes
- **Session Timeout**: Automatically closes inactive sessions
- **Database Persistence**: Tracks all sessions in PostgreSQL

**Session Lifecycle:**
1. **Create**: Session created on first access to a server in a conversation
2. **Initialize**: MCP handshake performed, capabilities negotiated
3. **Active**: Tools/resources cached, notifications monitored
4. **Inactive**: Session marked inactive after timeout
5. **Cleanup**: Session closed, database updated

## Configuration

MCP servers are configured in `config.yaml`:

```yaml
mcp_servers:
  # Local filesystem server (stdio)
  - name: filesystem
    type: stdio
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-filesystem"
      - "/Users/yourusername/Documents"
    enabled: true
    description: "Local filesystem access"

  # Remote HTTP server
  - name: sentry
    type: http
    url: "https://mcp.sentry.io"
    headers:
      Authorization: "Bearer ${SENTRY_TOKEN}"
    enabled: false
    description: "Sentry integration"
```

## Database Schema

The manager persists sessions and caches in PostgreSQL:

**Tables:**
- `mcp_sessions` - Active MCP server connections
  - Tracks connection info, capabilities, status
  - Links to conversation
  - Stores last active time

- `mcp_tools` - Cached tool definitions
  - Tool name, description, input schema
  - Links to session
  - Refreshed when `tools/list_changed` notification received

- `mcp_resources` - Cached resource metadata
  - Resource URI, name, description, mime type
  - Links to session
  - Refreshed when `resources/list_changed` notification received

## Integration with Agent

The agent will use the MCP manager to:

1. **Gather Tools**: Get all available tools for the active user
   ```go
   tools, err := mcpManager.ListAllToolsForUser(ctx, userID, bearerToken)
   ```

2. **Format for LLM**: Convert MCP tools to OpenAI function format
   ```go
   for _, toolWithServer := range tools {
       llmTools = append(llmTools, formatToolForLLM(toolWithServer))
   }
   ```

3. **Execute Tools**: When LLM requests tool execution
   ```go
   result, err := mcpManager.CallTool(
       ctx,
       conversationID,
       serverName,
       toolName,
       arguments,
   )
   ```

4. **Gather Context**: Read relevant resources for LLM context
   ```go
   resources, err := mcpManager.ListAllResourcesForUser(ctx, userID, bearerToken)
   for _, res := range resources {
       if isRelevant(res.Resource, userQuery) {
           content, _ := mcpManager.ReadResource(ctx, conversationID, res.ServerName, res.Resource.URI)
           // Add to LLM context
       }
   }
   ```

## Example: Using the Filesystem Server

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/d4l-data4life/go-mcp-host/pkg/config"
    "github.com/d4l-data4life/go-mcp-host/pkg/mcp/manager"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

func main() {
    // Assume db is initialized
    var db *gorm.DB
    
    // Create manager
    mcpManager := manager.NewMCPManager(db, 1*time.Hour)
    
    // Configure filesystem server
    serverConfig := config.MCPServerConfig{
        Name:    "filesystem",
        Type:    "stdio",
        Command: "npx",
        Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
        Enabled: true,
    }
    
	// Create session
	ctx := context.Background()
	conversationID := uuid.New()
	userID := uuid.New()
	bearerToken := ""

	session, err := mcpManager.GetOrCreateSession(ctx, conversationID, serverConfig, bearerToken, userID)
    if err != nil {
        panic(err)
    }
    
	// List tools for the user (cached per server)
	tools, err := mcpManager.ListAllToolsForUser(ctx, userID, bearerToken)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Found %d tools:\n", len(tools))
    for _, t := range tools {
        fmt.Printf("  - %s: %s\n", t.Tool.Name, t.Tool.Description)
    }
    
    // Call read_file tool
    result, err := mcpManager.CallTool(ctx, conversationID, "filesystem", "read_file", map[string]interface{}{
        "path": "/tmp/test.txt",
    })
    if err != nil {
        panic(err)
    }
    
    // Print result
    for _, content := range result.Content {
        fmt.Printf("File contents: %s\n", content.Text)
    }
    
    // Cleanup
    mcpManager.CloseAllSessionsForConversation(conversationID)
}
```

## Error Handling

The implementation uses Go's error wrapping with `github.com/pkg/errors`:

```go
if err != nil {
    return nil, errors.Wrap(err, "failed to initialize MCP client")
}
```

Common errors:
- `"transport not connected"` - Transport was closed or failed
- `"client not initialized"` - Forgot to call Initialize()
- `"no active session for server X"` - Server not configured or session died
- `"JSON-RPC error CODE: MESSAGE"` - Server returned an error

## Testing

To test the MCP implementation:

```bash
# Start Ollama (for later integration)
ollama serve

# Start PostgreSQL
make docker-database

# Test with a simple MCP server
npx @modelcontextprotocol/inspector npx -y @modelcontextprotocol/server-filesystem /tmp

# Build and run the service
make run
```

## Next Steps

With the MCP implementation complete, the next phase is to:

1. **Implement Ollama Client** - Connect to Ollama for LLM capabilities
2. **Implement Agent Orchestrator** - Coordinate MCP tools with LLM reasoning
3. **Implement API Handlers** - Expose MCP functionality via REST API
4. **Build Frontend** - Create chat interface in React Native

See `TODO.md` for detailed task breakdown.

## References

- [MCP Specification](https://modelcontextprotocol.io/docs/specification)
- [mark3labs/mcphost](https://github.com/mark3labs/mcphost) - Reference implementation
- [Official MCP Servers](https://github.com/modelcontextprotocol/servers)
