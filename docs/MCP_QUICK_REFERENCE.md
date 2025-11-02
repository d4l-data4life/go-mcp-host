# MCP (Model Context Protocol) Quick Reference

## What is MCP?

Model Context Protocol (MCP) is an open protocol that standardizes how applications provide context to Large Language Models (LLMs). It enables AI applications like Claude, IDEs, and other tools to connect to external data sources and tools through a unified interface.

**Official Documentation**: https://modelcontextprotocol.io/docs/learn/architecture

---

## Core Concepts

### Architecture

```
┌─────────────────┐
│   MCP Host      │  (Our go-mcp-host service)
│  (AI App)       │
└────────┬────────┘
         │ manages
         ▼
┌─────────────────┐
│   MCP Client    │  (1:1 with each server)
│   (per server)  │
└────────┬────────┘
         │ connects to
         ▼
┌─────────────────┐
│   MCP Server    │  (Filesystem, Sentry, etc.)
│   (provides     │
│    context)     │
└─────────────────┘
```

### Participants

1. **MCP Host**: The AI application (our Go service) that coordinates one or multiple MCP clients
2. **MCP Client**: Component that maintains a 1:1 connection to an MCP server
3. **MCP Server**: Program that provides context (tools, resources, prompts) to clients

**Key Point**: Our `go-mcp-host` is an **MCP Host** that creates **MCP Clients** to connect to external **MCP Servers**.

---

## Protocol Layers

### 1. Transport Layer (How to Connect)

Two transport mechanisms:

#### Stdio Transport (Local Servers)
- Uses standard input/output streams
- For local processes on same machine
- Zero network overhead
- Example: filesystem server, local database tools

```go
// Example: Launch filesystem server via stdio
cmd := exec.Command("npx", "-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir")
stdin, _ := cmd.StdinPipe()
stdout, _ := cmd.StdoutPipe()
cmd.Start()
```

#### HTTP Transport (Remote Servers)
- HTTP POST for client → server messages
- Server-Sent Events (SSE) for server → client messages
- Supports bearer tokens, API keys, OAuth
- Example: Sentry MCP server, cloud APIs

```go
// Example: Connect to remote server
client := NewHTTPMCPClient("https://api.example.com/mcp", bearerToken)
```

### 2. Data Layer (What to Exchange)

Uses **JSON-RPC 2.0** for all communication.

#### Message Types

**Request** (expects response):
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list",
  "params": {}
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [...]
  }
}
```

**Notification** (no response):
```json
{
  "jsonrpc": "2.0",
  "method": "notifications/tools/list_changed"
}
```

---

## MCP Primitives

### Server Primitives (What Servers Provide)

#### 1. Tools
Executable functions that AI can invoke to perform actions.

**Methods**:
- `tools/list` - Discover available tools
- `tools/call` - Execute a tool

**Example Tool**:
```json
{
  "name": "read_file",
  "description": "Read contents of a file",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": { "type": "string" }
    },
    "required": ["path"]
  }
}
```

**Use Case**: File operations, API calls, database queries, web searches

#### 2. Resources
Data sources that provide contextual information to AI.

**Methods**:
- `resources/list` - Discover available resources
- `resources/read` - Retrieve a resource

**Example Resource**:
```json
{
  "uri": "file:///path/to/schema.sql",
  "name": "Database Schema",
  "description": "PostgreSQL database schema",
  "mimeType": "text/plain"
}
```

**Use Case**: File contents, database schemas, API documentation, configuration data

#### 3. Prompts
Reusable templates for structuring LLM interactions.

**Methods**:
- `prompts/list` - Discover available prompts
- `prompts/get` - Retrieve a prompt

**Example Prompt**:
```json
{
  "name": "code_review",
  "description": "Review code changes",
  "arguments": [
    {
      "name": "language",
      "description": "Programming language",
      "required": true
    }
  ]
}
```

**Use Case**: System prompts, few-shot examples, templated instructions

### Client Primitives (What Clients Can Expose)

#### 1. Sampling
Allows servers to request LLM completions from the client.

**Method**: `sampling/createMessage`

**Use Case**: Server needs AI assistance but wants to stay model-agnostic

#### 2. Roots
Allows servers to request file system roots from the client.

**Method**: `roots/list`

**Use Case**: File system servers need to know what directories to access

---

## Lifecycle Management

### Connection Initialization

1. **Client connects** to server (via stdio or HTTP)
2. **Initialize exchange**: Negotiate capabilities
3. **Client sends** `initialized` notification
4. **Connection ready** for use

```json
// 1. Client → Server: Initialize request
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "roots": { "listChanged": true },
      "sampling": {}
    },
    "clientInfo": {
      "name": "go-mcp-host",
      "version": "1.0.0"
    }
  }
}

// 2. Server → Client: Initialize response
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": { "listChanged": true },
      "resources": { "subscribe": true }
    },
    "serverInfo": {
      "name": "filesystem-server",
      "version": "1.0.0"
    }
  }
}

// 3. Client → Server: Initialized notification
{
  "jsonrpc": "2.0",
  "method": "notifications/initialized"
}
```

### Capability Negotiation

Both client and server declare what they support:

**Client Capabilities**:
- `sampling` - Can provide LLM completions
- `roots` - Can provide file system roots

**Server Capabilities**:
- `tools` - Provides tools (with `listChanged` for notifications)
- `resources` - Provides resources (with `subscribe` for updates)
- `prompts` - Provides prompts
- `logging` - Supports logging

### Notifications

Servers can notify clients about changes:

- `notifications/tools/list_changed` - Tool list updated
- `notifications/resources/list_changed` - Resource list updated
- `notifications/resources/updated` - Specific resource content changed

**Client should respond** by re-fetching the list.

---

## Typical Workflow in Our Agent

### 1. User Sends Message

```
User: "What files are in my project directory?"
```

### 2. Agent Gathers Context

```go
// List all available tools from all connected MCP servers
tools := mcpManager.ListAllTools(ctx, conversationID)
// Tools: [list_directory, read_file, write_file, ...]

// Check for relevant resources
resources := mcpManager.ListAllResources(ctx, conversationID)
// Resources: [project_structure.md, README.md, ...]
```

### 3. Agent Builds LLM Prompt

```go
// Convert MCP tools to OpenAI function format
llmTools := []Tool{
  {
    Name: "list_directory",
    Description: "List contents of a directory",
    Parameters: {...}
  },
  ...
}

// Build messages
messages := []Message{
  {Role: "system", Content: "You are a helpful assistant with access to filesystem tools."},
  {Role: "user", Content: "What files are in my project directory?"},
}

// Send to LLM
response := ollamaClient.Chat(ctx, messages, llmTools)
```

### 4. LLM Decides to Use Tool

```json
{
  "role": "assistant",
  "content": null,
  "tool_calls": [
    {
      "id": "call_1",
      "type": "function",
      "function": {
        "name": "list_directory",
        "arguments": "{\"path\": \"/project\"}"
      }
    }
  ]
}
```

### 5. Agent Executes Tool via MCP

```go
// Find which MCP server provides this tool
mcpClient := mcpManager.GetClientForTool("list_directory")

// Execute tool via MCP protocol
result := mcpClient.CallTool(ctx, "list_directory", map[string]interface{}{
  "path": "/project",
})
// Result: {content: [{type: "text", text: "file1.go\nfile2.go\n..."}]}
```

### 6. Agent Returns Results to LLM

```go
// Add tool result to conversation
messages = append(messages, Message{
  Role: "tool",
  ToolCallID: "call_1",
  Content: "file1.go\nfile2.go\nREADME.md\n...",
})

// Ask LLM to synthesize response
finalResponse := ollamaClient.Chat(ctx, messages, llmTools)
```

### 7. LLM Responds to User

```
Assistant: "Your project directory contains the following files:
- file1.go
- file2.go  
- README.md
..."
```

---

## Implementation Checklist for go-mcp-host

### Phase 1: MCP Client Basics
- [ ] JSON-RPC message structures (Request, Response, Notification, Error)
- [ ] Transport interface and implementations (stdio, HTTP)
- [ ] Initialize/capability negotiation
- [ ] Basic tool discovery and execution

### Phase 2: Full MCP Support
- [ ] Resource discovery and reading
- [ ] Prompt discovery and retrieval
- [ ] Notification handlers (list_changed events)
- [ ] Sampling support (client primitive)
- [ ] Roots support (client primitive)

### Phase 3: Session Management
- [ ] MCP client lifecycle (spawn, connect, use, cleanup)
- [ ] Session pooling per user/conversation
- [ ] Reconnection logic for dropped connections
- [ ] Capability caching

### Phase 4: Integration with Agent
- [ ] Tool registry (aggregate from all servers)
- [ ] Context builder (gather relevant resources)
- [ ] Tool executor (route to correct MCP client)
- [ ] Bearer token propagation

---

## Example MCP Servers

### Official Servers
1. **Filesystem Server**: Read/write local files
   ```bash
   npx -y @modelcontextprotocol/server-filesystem /path/to/allowed/dir
   ```

2. **PostgreSQL Server**: Query databases
   ```bash
   npx -y @modelcontextprotocol/server-postgres postgresql://localhost/dbname
   ```

3. **Puppeteer Server**: Web automation
   ```bash
   npx -y @modelcontextprotocol/server-puppeteer
   ```

4. **Sentry Server**: Access Sentry issues
   ```
   Remote HTTP server: https://mcp.sentry.io
   ```

### Community Servers
- GitHub integration
- Slack integration  
- Google Drive
- And many more...

**Registry**: https://github.com/modelcontextprotocol/servers

---

## Key Differences from Other Protocols

### MCP vs Function Calling
- **Function Calling**: LLM-specific feature for executing functions
- **MCP**: Standardized protocol for providing context AND tools to ANY AI application

### MCP vs RAG
- **RAG**: Retrieve documents and embed in prompt
- **MCP**: Dynamic tool execution + resource access + prompt templates

### MCP vs LangChain Tools
- **LangChain**: Python/JS-specific tool framework
- **MCP**: Language-agnostic protocol, any language can implement

---

## Best Practices

### 1. Connection Management
- Reuse MCP clients when possible (pool per conversation)
- Handle disconnections gracefully (retry, reconnect)
- Cleanup connections when conversation ends

### 2. Error Handling
- MCP servers can fail - always have fallbacks
- Invalid tool arguments - validate before sending
- Timeout tool calls - don't wait forever

### 3. Security
- Validate tool inputs (prevent injection attacks)
- Propagate user auth tokens to remote MCP servers
- Restrict tool access based on user permissions
- Sanitize tool outputs before sending to LLM

### 4. Performance
- Cache tool/resource lists (refresh on notifications)
- Limit context size (only include relevant resources)
- Batch tool calls when possible
- Use streaming for long-running tools

### 5. User Experience
- Show tool execution in UI (transparency)
- Allow users to approve dangerous tools (file writes, deletions)
- Provide tool descriptions in UI
- Log all tool executions (audit trail)

---

## Debugging Tips

### 1. Enable Debug Logging
```bash
export GO_SVC_TEMPLATE_DEBUG=true
```

### 2. Test MCP Server Separately
Use the official MCP Inspector tool:
```bash
npx @modelcontextprotocol/inspector [server-command]
```

### 3. Check JSON-RPC Messages
Log all messages sent/received for debugging:
```go
logging.LogDebugf("MCP Request: %s", string(requestJSON))
logging.LogDebugf("MCP Response: %s", string(responseJSON))
```

### 4. Common Issues
- **Tool not found**: Check tool name matches exactly
- **Connection timeout**: Verify server is running and accessible
- **Invalid JSON**: Ensure proper JSON-RPC 2.0 format
- **Capability mismatch**: Check initialization exchange

---

## Resources

- **MCP Specification**: https://modelcontextprotocol.io/docs/specification
- **MCP GitHub**: https://github.com/modelcontextprotocol
- **MCP Community**: https://github.com/modelcontextprotocol/servers
- **MCP Inspector**: https://modelcontextprotocol.io/docs/tools/inspector
- **Ollama API**: https://github.com/ollama/ollama/blob/main/docs/api.md

