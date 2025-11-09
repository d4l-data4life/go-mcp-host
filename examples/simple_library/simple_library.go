package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/d4l-data4life/go-mcp-host/pkg/config"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcphost"
	"github.com/d4l-data4life/go-mcp-host/pkg/models"
)

// This example demonstrates the simplest way to use go-mcp-host as a library
// using the high-level mcphost package API

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

	// Create MCP Host with configuration
	host, err := mcphost.NewHost(context.Background(), mcphost.Config{
		MCPServers: []config.MCPServerConfig{
			{
				Name:        "weather",
				Type:        "stdio",
				Command:     "npx",
				Args:        []string{"-y", "@h1deya/mcp-server-weather"},
				Enabled:     true,
				Description: "Weather information server",
			},
			{
				Name:        "sequential-thinking",
				Type:        "stdio",
				Command:     "npx",
				Args:        []string{"-y", "@modelcontextprotocol/server-sequential-thinking"},
				Enabled:     true,
				Description: "Sequential thinking server for complex reasoning",
			},
		},
		OpenAIBaseURL:      "http://localhost:11434",
		OpenAIDefaultModel: "llama3.2",
		DB:                 db,
		// Optional: Customize agent behavior
		AgentConfig: mcphost.Config{}.AgentConfig, // Uses defaults
	})
	if err != nil {
		log.Fatalf("Failed to create MCP Host: %v", err)
	}

	fmt.Println("✅ MCP Host initialized successfully!")
	fmt.Println()

	// Example 1: Simple chat request
	fmt.Println("=== Example 1: Simple Chat ===")
	conversationID := uuid.New()
	userID := uuid.New()

	response, err := host.Chat(context.Background(), mcphost.ChatRequest{
		ConversationID: conversationID,
		UserID:         userID,
		UserMessage:    "What's the weather in San Diego?",
	})

	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Response: %s\n", response.Message.Content)
		fmt.Printf("Tools used: %d\n", len(response.ToolsUsed))
		fmt.Printf("Iterations: %d\n", response.Iterations)
	}
	fmt.Println()

	// Example 2: Streaming chat
	fmt.Println("=== Example 2: Streaming Chat ===")
	streamChan, err := host.ChatStream(context.Background(), mcphost.ChatRequest{
		ConversationID: conversationID,
		UserID:         userID,
		UserMessage:    "Compare the weather in San Diego and New York",
	})

	if err != nil {
		log.Printf("Error starting stream: %v", err)
	} else {
		fmt.Print("Response: ")
		for event := range streamChan {
			switch event.Type {
			case mcphost.StreamEventTypeContent:
				fmt.Print(event.Content)
			case mcphost.StreamEventTypeToolStart:
				fmt.Printf("\n[🔧 Using tool: %s]\n", event.Tool.ToolName)
			case mcphost.StreamEventTypeToolComplete:
				if event.Tool.Error != nil {
					fmt.Printf("[❌ Tool failed: %v]\n", event.Tool.Error)
				} else {
					fmt.Printf("[✅ Tool complete in %v]\n", event.Tool.Duration)
				}
			case mcphost.StreamEventTypeDone:
				fmt.Println("\n[Done]")
			case mcphost.StreamEventTypeError:
				fmt.Printf("\n[Error: %v]\n", event.Error)
			}
		}
	}
	fmt.Println()

	// Example 3: List available tools
	fmt.Println("=== Example 3: List Available Tools ===")
	tools, err := host.GetAvailableTools(context.Background(), userID, "")
	if err != nil {
		log.Printf("Error getting tools: %v", err)
	} else {
		fmt.Printf("Found %d tools:\n", len(tools))
		for _, tool := range tools {
			fmt.Printf("  • %s.%s: %s\n", tool.ServerName, tool.ToolName, tool.Description)
		}
	}
	fmt.Println()

	// Cleanup
	fmt.Println("Cleaning up...")
	if err := host.CloseConversation(conversationID); err != nil {
		log.Printf("Warning: Failed to close conversation: %v", err)
	}

	fmt.Println("✅ Done!")
}
