package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/d4l-data4life/go-mcp-host/pkg/config"
	"github.com/d4l-data4life/go-mcp-host/pkg/llm"
	llmopenai "github.com/d4l-data4life/go-mcp-host/pkg/llm/openai"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/manager"
)

// This example demonstrates how to use an OpenAI-compatible endpoint (like Ollama)
// with MCP tools. It shows the complete flow: MCP tools → LLM → Tool execution → Response

func main() {
	// Setup configuration
	config.SetupEnv()
	config.SetupLogger()

	// Note: This example doesn't use the database for simplicity,
	// but in production you'd want to connect to persist conversations
	// dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
	// 	"localhost", "6000", "go-mcp-host", "postgres", "go-mcp-host")
	// db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	// if err != nil {
	// 	fmt.Printf("Failed to connect to database: %v\n", err)
	// 	os.Exit(1)
	// }

	// Configure weather MCP server
	weatherConfig := config.MCPServerConfig{
		Name:    "weather",
		Type:    "stdio",
		Command: "npx",
		Args:    []string{"-y", "@h1deya/mcp-server-weather"},
		Enabled: true,
	}

	// Create MCP manager
	mcpServers := []config.MCPServerConfig{weatherConfig}
	mcpManager := manager.NewManager(mcpServers)

	// Create OpenAI-compatible client pointed at the local Ollama endpoint
	ollamaClient := llmopenai.NewClient(llmopenai.Config{
		BaseURL: "http://localhost:11434", // Ollama's default
		Model:   "llama3.2",
		Timeout: 5 * time.Minute,
	})

	// Test connection
	fmt.Println("Testing OpenAI-compatible endpoint...")
	models, err := ollamaClient.ListModels(context.Background())
	if err != nil {
		fmt.Printf("Failed to connect to the endpoint: %v\n", err)
		fmt.Println("Make sure Ollama is running: ollama serve")
		os.Exit(1)
	}
	fmt.Printf("Connected successfully. Available models: %d\n", len(models))
	for _, model := range models {
		fmt.Printf("  - %s\n", model.Name)
	}

	// Create a conversation
	conversationID := uuid.New()
	userID := uuid.New()
	fmt.Printf("\nConversation ID: %s\n", conversationID)

	// Create MCP session
	fmt.Println("\nInitializing weather MCP server...")
	ctx := context.Background()
	session, err := mcpManager.GetOrCreateSession(ctx, conversationID, weatherConfig, "", userID)
	if err != nil {
		fmt.Printf("Failed to create MCP session: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("MCP session created: %s\n", session.SessionID)

	// Wait a moment for tools to be discovered
	time.Sleep(2 * time.Second)

	// Get all available tools
	toolsWithServer, err := mcpManager.GetAllTools(ctx, conversationID)
	if err != nil {
		fmt.Printf("Failed to get tools: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nDiscovered %d tools:\n", len(toolsWithServer))
	for _, t := range toolsWithServer {
		fmt.Printf("  - %s.%s: %s\n", t.ServerName, t.Tool.Name, t.Tool.Description)
	}

	// Convert MCP tools to LLM format
	var (
		llmTools   []llm.Tool
		toolLookup = make(map[string]manager.ToolWithServer, len(toolsWithServer))
	)
	for _, t := range toolsWithServer {
		llmTool := llm.ConvertMCPToolToLLMTool(t.Tool, t.ServerName)
		llmTools = append(llmTools, llmTool)
		toolLookup[llmTool.Function.Name] = t
	}

	// Create a chat request with tools
	fmt.Println("\nSending request to LLM...")
	messages := []llm.Message{
		{
			Role:    llm.RoleSystem,
			Content: "You are a helpful assistant with access to weather information. When asked about weather, use the available tools to get current data.",
		},
		{
			Role:    llm.RoleUser,
			Content: "What's the weather like in San Francisco?",
		},
	}

	chatRequest := llm.ChatRequest{
		Model:    "llama3.2",
		Messages: messages,
		Tools:    llmTools,
	}

	// Send to LLM
	response, err := ollamaClient.Chat(ctx, chatRequest)
	if err != nil {
		fmt.Printf("Failed to get LLM response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nLLM Response:\n")
	fmt.Printf("  Role: %s\n", response.Message.Role)
	fmt.Printf("  Content: %s\n", response.Message.Content)
	fmt.Printf("  Tool Calls: %d\n", len(response.Message.ToolCalls))

	// Execute tool calls if any
	if len(response.Message.ToolCalls) > 0 {
		fmt.Println("\nExecuting tool calls...")

		for _, toolCall := range response.Message.ToolCalls {
			fmt.Printf("\n  Tool: %s\n", toolCall.Function.Name)

			// Resolve server/tool names from lookup map
			binding, ok := toolLookup[toolCall.Function.Name]
			if !ok {
				fmt.Printf("  Error: unknown tool %s\n", toolCall.Function.Name)
				continue
			}
			serverName := binding.ServerName
			toolName := binding.Tool.Name
			fmt.Printf("  Server: %s\n", serverName)
			fmt.Printf("  Method: %s\n", toolName)

			// Parse arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				fmt.Printf("  Error parsing arguments: %v\n", err)
				continue
			}
			fmt.Printf("  Arguments: %v\n", args)

			// Execute via MCP
			result, err := mcpManager.CallTool(ctx, conversationID, serverName, toolName, args)
			if err != nil {
				fmt.Printf("  Error: %v\n", err)
				continue
			}

			// Convert result to string
			resultText := llm.ConvertMCPContentToString(result.Content)
			fmt.Printf("  Result: %s\n", resultText)

			// Add tool result to messages for next LLM call
			messages = append(messages, llm.Message{
				Role:      llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{toolCall},
			})
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: toolCall.ID,
				Content:    resultText,
			})
		}

		// Send tool results back to LLM for final response
		fmt.Println("\nSending tool results back to LLM...")
		chatRequest.Messages = messages
		finalResponse, err := ollamaClient.Chat(ctx, chatRequest)
		if err != nil {
			fmt.Printf("Failed to get final response: %v\n", err)
		} else {
			fmt.Printf("\nFinal Response:\n%s\n", finalResponse.Message.Content)
		}
	}

	// Cleanup
	fmt.Println("\nCleaning up...")
	mcpManager.CloseAllSessionsForConversation(conversationID)
	fmt.Println("Done!")
}
