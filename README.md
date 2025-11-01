# go-mcp-host

A Go-based Model Context Protocol (MCP) Host service with AI agent capabilities. This service acts as an intelligent agent that connects to multiple MCP servers, integrates with Ollama (OpenAI-compatible LLM), and provides an agentic AI experience to users.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## Features

- **MCP Protocol Support**: Full implementation of the Model Context Protocol
  - Stdio and HTTP transports
  - Tools, Resources, and Prompts primitives
  - Dynamic capability negotiation
  - Notification handling (list_changed events)
  
- **AI Agent Orchestration**: Intelligent reasoning loop
  - Context gathering from MCP resources
  - Tool discovery and execution
  - Multi-iteration planning and execution
  - Error recovery

- **LLM Integration**: OpenAI-compatible API
  - Ollama support out of the box
  - Streaming responses
  - Function calling for tool execution
  
- **Conversation Management**: Full persistence
  - PostgreSQL-backed conversation history
  - Multi-user support with authentication
  - Session lifecycle management

- **Production Ready**
  - RESTful API with WebSocket streaming
  - JWT authentication support
  - Kubernetes deployment via Helm
  - Prometheus metrics
  - Structured logging

## Quick Start

### Option 1: Use as a Standalone Service

Deploy go-mcp-host as a microservice in your infrastructure.

#### Prerequisites

- Ollama running locally or remotely
- PostgreSQL database
- (Optional) Kubernetes cluster for production deployment

#### Run Locally

```bash
# Clone the repository
git clone https://github.com/d4l-data4life/go-mcp-host.git
cd go-mcp-host

# Copy and configure
cp config.example.yaml config.yaml
# Edit config.yaml to add your MCP servers

# Start PostgreSQL
make docker-database

# Run the service
make run
```

The service will be available at `http://localhost:8080`.

#### Deploy to Kubernetes

```bash
# See detailed deployment guide
cd deploy
cat README.md

# Quick deploy
helm install go-mcp-host ./helm-chart \
  -f examples/local/values.yaml \
  --namespace mcp-host
```

See [deploy/README.md](deploy/README.md) for full deployment documentation.

### Option 2: Use as a Go Library

Embed MCP Host functionality into your own Go application.

#### Installation

```bash
go get github.com/d4l-data4life/go-mcp-host
```

#### Basic Usage

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
    // Setup database
    db, err := gorm.Open(postgres.Open("host=localhost port=5432 user=mcphost dbname=mcphost password=postgres sslmode=disable"), &gorm.Config{})
    if err != nil {
        log.Fatal(err)
    }

    // Create MCP Host
    host, err := mcphost.NewHost(context.Background(), mcphost.Config{
        MCPServers: []config.MCPServerConfig{
            {
                Name:    "weather",
                Type:    "stdio",
                Command: "npx",
                Args:    []string{"-y", "@h1deya/mcp-server-weather"},
                Enabled: true,
                Description: "Weather information server",
            },
        },
        LLMEndpoint: "http://localhost:11434",
        DB:          db,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Chat with the agent
    response, err := host.Chat(context.Background(), mcphost.ChatRequest{
        ConversationID: uuid.New(),
        UserID:         uuid.New(),
        UserMessage:    "What's the weather in New York?",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(response.Message.Content)
}
```

See [examples/](examples/) for more usage examples.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Client (React/API)                    │
└─────────────────────────┬───────────────────────────────────┘
                          │ HTTP/WebSocket
┌─────────────────────────▼───────────────────────────────────┐
│                      go-mcp-host                             │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Agent Orchestrator                                     │ │
│  │  - Context gathering                                    │ │
│  │  - Tool execution loop                                  │ │
│  │  - Response generation                                  │ │
│  └───────────┬────────────────────────┬────────────────────┘ │
│              │                        │                      │
│  ┌───────────▼──────────┐  ┌─────────▼──────────┐          │
│  │  MCP Manager         │  │  LLM Client        │          │
│  │  - Session mgmt      │  │  - Ollama API      │          │
│  │  - Client pooling    │  │  - Function calls  │          │
│  └───────────┬──────────┘  └────────────────────┘          │
│              │                                               │
└──────────────┼───────────────────────────────────────────────┘
               │
    ┌──────────┼──────────┐
    │          │          │
┌───▼────┐ ┌──▼────┐ ┌──▼────┐
│ MCP    │ │ MCP   │ │ MCP   │
│Server 1│ │Server2│ │Server3│
│(stdio) │ │(HTTP) │ │(stdio)│
└────────┘ └───────┘ └───────┘
```

## Configuration

### MCP Servers

Configure MCP servers in `config.yaml`:

```yaml
mcp_servers:
  # Stdio server example
  - name: weather
    type: stdio
    command: npx
    args:
      - "-y"
      - "@h1deya/mcp-server-weather"
    enabled: true
    description: "Weather information server"
  
  # HTTP server example
  - name: my-api
    type: http
    url: "https://api.example.com/mcp"
    headers:
      X-API-Key: "your-api-key"
    forwardBearer: true  # Forward user's bearer token
    enabled: true
    description: "My custom MCP server"
```

### Environment Variables

Key environment variables:

- `PORT` - HTTP port (default: 8080)
- `CORS_HOSTS` - Allowed CORS origins
- `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS` - Database connection
- `REMOTE_KEYS_URL` - JWT validation endpoint (optional)
- `DEBUG` - Enable debug logging

See [config.example.yaml](config.example.yaml) for all options.

## API Documentation

### REST Endpoints

- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - Login
- `GET /api/v1/conversations` - List conversations
- `POST /api/v1/conversations` - Create conversation
- `DELETE /api/v1/conversations/:id` - Delete conversation
- `POST /api/v1/messages` - Send message
- `WS /api/v1/messages/stream` - Stream responses
- `GET /api/v1/mcp/servers` - List MCP servers
- `GET /api/v1/mcp/tools` - List available tools

See [swagger/api.yml](swagger/api.yml) for the full API specification.

## Development

### Prerequisites

- Go 1.24+
- Docker & Docker Compose
- PostgreSQL
- Ollama
- golangci-lint (for linting)

### Building

```bash
# Build binary
make build

# Build Docker image
make docker-build

# Run tests
make test

# Run linter
make lint
```

### Project Structure

```
go-mcp-host/
├── cmd/
│   └── api/              # Main application entry point
├── pkg/
│   ├── agent/            # Agent orchestration logic
│   ├── mcp/              # MCP protocol implementation
│   │   ├── client/       # MCP client core
│   │   ├── manager/      # Session management
│   │   ├── protocol/     # Protocol types
│   │   └── transport/    # Stdio/HTTP transports
│   ├── llm/              # LLM integration (Ollama)
│   ├── handlers/         # HTTP handlers
│   ├── models/           # Database models
│   ├── config/           # Configuration
│   ├── auth/             # Authentication
│   ├── server/           # HTTP server setup
│   └── mcphost/          # Public API for library usage
├── deploy/
│   ├── helm-chart/       # Kubernetes Helm chart
│   └── examples/         # Example configurations
├── docs/                 # Additional documentation
├── examples/             # Usage examples
├── sql/                  # Database migrations
└── swagger/              # API specification
```

## Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run integration tests (requires running database)
make docker-database
make test-integration
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Code Standards

- Follow standard Go conventions
- Run `gofmt` and `golangci-lint`
- Write tests for new features
- Update documentation

See [.cursorrules](.cursorrules) for detailed coding standards.

## Documentation

- [MCP Implementation Guide](docs/MCP_IMPLEMENTATION.md)
- [MCP Quick Reference](docs/MCP_QUICK_REFERENCE.md)
- [Deployment Guide](deploy/README.md)
- [API Specification](swagger/api.yml)

## MCP Resources

- [Model Context Protocol Specification](https://modelcontextprotocol.io/docs/specification)
- [MCP Architecture](https://modelcontextprotocol.io/docs/learn/architecture)
- [Available MCP Servers](https://github.com/modelcontextprotocol)

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.

## Acknowledgments

- Built with [go-svc](https://github.com/d4l-data4life/go-svc) framework
- Powered by [Ollama](https://ollama.ai/)
- Implements [Model Context Protocol](https://modelcontextprotocol.io/)

## Support

- GitHub Issues: [Report bugs or request features](https://github.com/d4l-data4life/go-mcp-host/issues)
- Documentation: [Full documentation](https://github.com/d4l-data4life/go-mcp-host/docs)

---

Made with ❤️ by Data4Life
