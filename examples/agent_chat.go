package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/d4l-data4life/go-mcp-host/pkg/agent"
	"github.com/d4l-data4life/go-mcp-host/pkg/config"
	"github.com/d4l-data4life/go-mcp-host/pkg/llm/ollama"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/manager"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// This example demonstrates using the Agent package for simplified interactions
// It replaces the manual orchestration from ollama_with_mcp.go

func main() {
	// Setup configuration
	config.SetupEnv()
	config.SetupLogger()

	// Connect to database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		"localhost", "6000", "go-mcp-host", "postgres", "go-mcp-host")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	// Create MCP manager
	mcpManager := manager.NewManager(db, 1*time.Hour)

	// Create Ollama client
	ollamaClient := ollama.NewClient(ollama.Config{
		BaseURL: "http://localhost:11434",
		Model:   "llama3.2",
		Timeout: 5 * time.Minute,
	})

	// Test Ollama connection
	fmt.Println("Testing Ollama connection...")
	models, err := ollamaClient.ListModels(context.Background())
	if err != nil {
		fmt.Printf("Failed to connect to Ollama: %v\n", err)
		fmt.Println("Make sure Ollama is running: ollama serve")
		os.Exit(1)
	}
	fmt.Printf("Connected to Ollama. Available models: %d\n", len(models))

	// Create Agent
	temp := 0.7
	maxTokens := 4096
	topP := 0.95

	agentConfig := agent.Config{
		MaxIterations:        10,
		MaxContextTokens:     8192,
		ToolExecutionTimeout: 60 * time.Second,
		SystemPrompt:         "You are a helpful AI assistant with access to weather information. When asked about weather, use the available tools to get current data.",
		DefaultModel:         "llama3.2",
		Temperature:          &temp,
		MaxTokens:            &maxTokens,
		TopP:                 &topP,
	}

	aiAgent := agent.NewAgent(db, mcpManager, ollamaClient, agentConfig)

	// Create a conversation
	conversationID := uuid.New()
	fmt.Printf("\nConversation ID: %s\n", conversationID)

	// Example 1: Non-streaming chat
	fmt.Println("\n=== Example 1: Non-Streaming Chat ===")
	response, err := aiAgent.Chat(context.Background(), agent.ChatRequest{
		ConversationID: conversationID,
		UserMessage:    "What's the weather like in San Francisco?",
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\nFinal Response:\n%s\n", response.Message.Content)
		fmt.Printf("\nStats:\n")
		fmt.Printf("  Iterations: %d\n", response.Iterations)
		fmt.Printf("  Tools Used: %d\n", len(response.ToolsUsed))
		fmt.Printf("  Total Tokens: %d\n", response.TotalTokens)

		if len(response.ToolsUsed) > 0 {
			fmt.Println("\nTools Executed:")
			for i, tool := range response.ToolsUsed {
				fmt.Printf("  %d. %s.%s", i+1, tool.ServerName, tool.ToolName)
				if tool.Error != nil {
					fmt.Printf(" - ERROR: %v\n", tool.Error)
				} else {
					fmt.Printf(" - Duration: %v\n", tool.Duration)
				}
			}
		}
	}

	// Example 2: Streaming chat
	fmt.Println("\n\n=== Example 2: Streaming Chat ===")
	streamChan, err := aiAgent.ChatStream(context.Background(), agent.ChatRequest{
		ConversationID: conversationID,
		UserMessage:    "How about New York?",
	})

	if err != nil {
		fmt.Printf("Error starting stream: %v\n", err)
	} else {
		fmt.Print("Response: ")
		for event := range streamChan {
			switch event.Type {
			case agent.StreamEventTypeContent:
				fmt.Print(event.Content)
			case agent.StreamEventTypeToolStart:
				fmt.Printf("\n[Using tool: %s]\n", event.Tool.ToolName)
			case agent.StreamEventTypeToolComplete:
				if event.Tool.Error != nil {
					fmt.Printf("[Tool failed: %v]\n", event.Tool.Error)
				} else {
					fmt.Printf("[Tool complete in %v]\n", event.Tool.Duration)
				}
			case agent.StreamEventTypeDone:
				fmt.Println("\n[Done]")
			case agent.StreamEventTypeError:
				fmt.Printf("\n[Error: %v]\n", event.Error)
			}
		}
	}

	// Example 3: List available tools
	fmt.Println("\n=== Available Tools ===")
	tools, err := aiAgent.GetAvailableTools(context.Background(), conversationID)
	if err != nil {
		fmt.Printf("Error getting tools: %v\n", err)
	} else {
		fmt.Printf("Found %d tools:\n", len(tools))
		for _, tool := range tools {
			fmt.Printf("  - %s.%s: %s\n", tool.ServerName, tool.ToolName, tool.Description)
		}
	}

	// Cleanup
	fmt.Println("\nCleaning up...")
	aiAgent.CloseConversation(conversationID)
	fmt.Println("Done!")
}
