package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/weese/go-mcp-host/pkg/agent"
	"github.com/weese/go-mcp-host/pkg/auth"
	"github.com/weese/go-mcp-host/pkg/config"
	"github.com/weese/go-mcp-host/pkg/handlers"
	"github.com/weese/go-mcp-host/pkg/llm/ollama"
	"github.com/weese/go-mcp-host/pkg/mcp/manager"

	"github.com/gesundheitscloud/go-svc/pkg/db"
	"github.com/gesundheitscloud/go-svc/pkg/logging"

	"github.com/go-chi/chi"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SetupRoutes adds all routes that the server should listen to
func SetupRoutes(ctx context.Context, mux *chi.Mux, tokenValidator auth.TokenValidator) {
	// Get database connection
	database := db.Get()

	// Initialize MCP Manager
	mcpConfig, err := config.LoadMCPConfig()
	if err != nil {
		logging.LogErrorf(err, "Failed to load MCP config, using defaults")
		mcpConfig = &config.FullMCPConfig{
			Servers: []config.MCPServerConfig{},
			Ollama:  config.GetOllamaConfig(),
			Agent:   config.GetAgentConfig(),
		}
	}

	mcpManager := manager.NewManager(mcpConfig.Servers)

	// Initialize Ollama client
	ollamaClient := ollama.NewClient(ollama.Config{
		BaseURL: mcpConfig.Ollama.BaseURL,
		Model:   mcpConfig.Ollama.DefaultModel,
	})

	// Initialize Agent
	agentInstance := agent.NewAgent(database, mcpManager, ollamaClient, agent.Config{
		MaxIterations: mcpConfig.Agent.MaxIterations,
		DefaultModel:  mcpConfig.Agent.DefaultModel,
	})

	// Register new API routes
	handlers.RegisterRoutes(mux, database, agentInstance, mcpManager, tokenValidator)

	// Health checks and metrics
	ch := handlers.NewChecksHandler()
	mux.Mount("/checks", ch.Routes())
	mux.Mount("/metrics", promhttp.Handler())

	// Displays all API paths when debug enabled
	walkFunc := func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.ReplaceAll(route, "/*/", "/")
		logging.LogDebugf("%s %s\n", method, route)
		return nil
	}
	if err := chi.Walk(mux, walkFunc); err != nil {
		logging.LogErrorf(err, "logging error")
	}
}
