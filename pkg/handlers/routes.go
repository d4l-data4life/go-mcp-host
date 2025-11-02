package handlers

import (
	"github.com/go-chi/chi"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"github.com/d4l-data4life/go-mcp-host/pkg/agent"
	"github.com/d4l-data4life/go-mcp-host/pkg/auth"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/manager"
)

// RegisterRoutes registers all API routes
func RegisterRoutes(r chi.Router, db *gorm.DB, agent *agent.Agent, mcpManager *manager.Manager, tokenValidator auth.TokenValidator) {
	jwtKey := []byte(viper.GetString("SERVICE_SECRET"))
	if len(jwtKey) == 0 {
		jwtKey = []byte("default-jwt-key-change-in-production")
	}

	// Public routes (no authentication required)
	authHandler := NewAuthHandler(db, jwtKey)
	r.Mount("/api/auth", authHandler.Routes())

	// Protected routes (authentication required)
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(jwtKey, db, tokenValidator))

		// Conversations
		conversationsHandler := NewConversationsHandler(db)
		r.Mount("/api/conversations", conversationsHandler.Routes())

		// Messages (nested under conversations)
		messagesHandler := NewMessagesHandler(db, agent)
		r.Route("/api/conversations/{id}/messages", func(r chi.Router) {
			r.Mount("/", messagesHandler.Routes())
		})

		// MCP Servers
		mcpServersHandler := NewMCPServersHandler(db, mcpManager)
		r.Mount("/api/mcp", mcpServersHandler.Routes())
	})

	// Note: Example handler uses old pattern with no DB parameter
	// exampleHandler := NewExampleHandler()
	// r.Mount("/api/examples", exampleHandler.Routes())
}
