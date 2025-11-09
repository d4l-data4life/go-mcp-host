package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/d4l-data4life/go-mcp-host/pkg/config"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcphost"
	"github.com/d4l-data4life/go-mcp-host/pkg/models"
)

// This example demonstrates how to embed go-mcp-host in your own web application
// You can integrate the MCP Host into your existing HTTP server

type Server struct {
	host *mcphost.Host
	db   *gorm.DB
}

func main() {
	// Setup database
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost port=6000 user=go-mcp-host password=postgres dbname=go-mcp-host sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate models
	if err := db.AutoMigrate(&models.User{}, &models.Conversation{}, &models.Message{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Create MCP Host
	host, err := mcphost.NewHost(context.Background(), mcphost.Config{
		MCPServers: []config.MCPServerConfig{
			{
				Name:        "weather",
				Type:        "stdio",
				Command:     "npx",
				Args:        []string{"-y", "@h1deya/mcp-server-weather"},
				Enabled:     true,
				Description: "Weather information",
			},
		},
		OpenAIBaseURL:      "http://localhost:11434",
		OpenAIDefaultModel: "llama3.2",
		DB:                 db,
	})
	if err != nil {
		log.Fatalf("Failed to create MCP Host: %v", err)
	}

	// Create server
	srv := &Server{
		host: host,
		db:   db,
	}

	// Setup routes
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Your existing application routes
	r.Get("/", srv.handleHome)
	r.Get("/health", srv.handleHealth)

	// MCP Host routes - embedded in your application
	r.Post("/api/chat", srv.handleChat)
	r.Post("/api/chat/stream", srv.handleChatStream)
	r.Get("/api/tools", srv.handleListTools)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 Server starting on port %s\n", port)
	fmt.Printf("   Try: curl -X POST http://localhost:%s/api/chat -d '{\"message\":\"What is the weather in NYC?\"}'\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	html := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>MCP Host Demo</title>
		<style>
			body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
			h1 { color: #333; }
			.chat-box { border: 1px solid #ddd; padding: 20px; margin: 20px 0; }
			input { width: 70%; padding: 10px; }
			button { padding: 10px 20px; background: #007bff; color: white; border: none; cursor: pointer; }
			button:hover { background: #0056b3; }
			#response { margin-top: 20px; padding: 15px; background: #f8f9fa; border-radius: 5px; }
		</style>
	</head>
	<body>
		<h1>MCP Host Demo</h1>
		<p>This is a simple web application with embedded go-mcp-host functionality.</p>
		
		<div class="chat-box">
			<h2>Chat with AI Agent</h2>
			<input type="text" id="message" placeholder="Ask about weather in any city..." />
			<button onclick="sendMessage()">Send</button>
			<div id="response"></div>
		</div>

		<script>
			async function sendMessage() {
				const message = document.getElementById('message').value;
				const responseDiv = document.getElementById('response');
				
				responseDiv.innerHTML = 'Thinking...';
				
				try {
					const response = await fetch('/api/chat', {
						method: 'POST',
						headers: {'Content-Type': 'application/json'},
						body: JSON.stringify({message})
					});
					
					const data = await response.json();
					responseDiv.innerHTML = '<strong>Response:</strong><br>' + data.response;
				} catch (error) {
					responseDiv.innerHTML = '<strong>Error:</strong> ' + error.message;
				}
			}
		</script>
	</body>
	</html>
	`
	fmt.Fprint(w, html)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Use MCP Host
	response, err := s.host.Chat(r.Context(), mcphost.ChatRequest{
		ConversationID: uuid.New(), // In real app, you'd track conversation IDs
		UserID:         uuid.New(), // In real app, you'd use authenticated user ID
		UserMessage:    req.Message,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response":   response.Message.Content,
		"tools_used": len(response.ToolsUsed),
		"iterations": response.Iterations,
	})
}

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Set headers for SSE (Server-Sent Events)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Use MCP Host streaming
	streamChan, err := s.host.ChatStream(r.Context(), mcphost.ChatRequest{
		ConversationID: uuid.New(),
		UserID:         uuid.New(),
		UserMessage:    req.Message,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	for event := range streamChan {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	// For demo purposes, create a temporary conversation
	conversationID := uuid.New()

	tools, err := s.host.GetAvailableTools(r.Context(), conversationID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": tools,
		"count": len(tools),
	})
}
