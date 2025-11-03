package handlers

import (
	"github.com/go-chi/chi"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"github.com/d4l-data4life/go-mcp-host/pkg/agent"
	"github.com/d4l-data4life/go-mcp-host/pkg/auth"
	"github.com/d4l-data4life/go-mcp-host/pkg/config"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/manager"
	"github.com/d4l-data4life/go-svc/pkg/middlewares"
)

// RegisterRoutes registers all API routes
func RegisterRoutes(
	r chi.Router,
	db *gorm.DB,
	agent *agent.Agent,
	mcpManager *manager.Manager,
	tokenValidator auth.TokenValidator,
	jwtSecret []byte,
) {
	prefix := viper.GetString("PREFIX")

	// External routes (ingress routes)
	r.Route(prefix, func(r chi.Router) {
		// Public routes (no authentication required)
		authHandler := NewAuthHandler(db, jwtSecret)
		r.Mount("/auth", authHandler.Routes())

		// Protected routes (authentication required)
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(db, tokenValidator))

			// Conversations
			conversationsHandler := NewConversationsHandler(db)
			r.Mount("/conversations", conversationsHandler.Routes())

			// Messages (nested under conversations)
			messagesHandler := NewMessagesHandler(db, agent)
			r.Route("/conversations/{id}/messages", func(r chi.Router) {
				r.Mount("/", messagesHandler.Routes())
			})

			// MCP Servers
			mcpServersHandler := NewMCPServersHandler(db, mcpManager)
			r.Mount("/mcp", mcpServersHandler.Routes())
		})
	})

	// Internal routes (service-to-service)
	r.Route(config.InternalPrefix, func(r chi.Router) {
		// Service-authenticated routes (require service secret)
		r.Group(func(r chi.Router) {
			// Get service secret from config
			serviceSecret := viper.GetString("SERVICE_SECRET")
			if serviceSecret == "" {
				// If no service secret is configured, skip service auth routes
				return
			}

			// Create service auth middleware with proper logger
			logger := NewServiceAuthLogger()
			serviceAuth := middlewares.NewServiceSecretAuthenticator(serviceSecret, logger)
			r.Use(serviceAuth.Authenticate())

			// Users management
			usersHandler := NewUsersHandler(db)
			r.Mount("/users", usersHandler.Routes())
		})
	})
}
