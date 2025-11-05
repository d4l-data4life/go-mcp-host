package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/d4l-data4life/go-mcp-host/pkg/config"
	"github.com/d4l-data4life/go-mcp-host/pkg/llm"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/manager"

	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// Orchestrator manages the agent's reasoning and tool execution loop
type Orchestrator struct {
	mcpManager *manager.Manager
	llmClient  llm.Client
	config     Config
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(mcpManager *manager.Manager, llmClient llm.Client, config Config) *Orchestrator {
	return &Orchestrator{
		mcpManager: mcpManager,
		llmClient:  llmClient,
		config:     config,
	}
}

// Execute runs the agent orchestration loop
func (o *Orchestrator) Execute(ctx context.Context, request ChatRequest) (*ChatResponse, error) {
	// Build initial messages
	messages := o.buildMessages(request)

	// Get available tools (short-lived clients, no sessions yet)
	toolsWithServer, err := o.mcpManager.ListAllToolsForUser(ctx, request.UserID, request.BearerToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tools")
	}

	// Convert to LLM format
	llmTools := make([]llm.Tool, 0, len(toolsWithServer))
	for _, t := range toolsWithServer {
		llmTools = append(llmTools, llm.ConvertMCPToolToLLMTool(t.Tool, t.ServerName))
	}

	logging.LogDebugf("Starting agent loop: tools=%d max_iterations=%d",
		len(llmTools), o.config.MaxIterations)

	// Execute reasoning loop
	var toolExecutions []ToolExecution
	var totalTokens int
	iteration := 0

	for iteration < o.config.MaxIterations {
		iteration++
		logging.LogDebugf("Agent iteration %d/%d", iteration, o.config.MaxIterations)

		// Build LLM request
		chatRequest := llm.ChatRequest{
			Model:       request.Model,
			Messages:    messages,
			Tools:       llmTools,
			Temperature: o.config.Temperature,
			MaxTokens:   o.config.MaxTokens,
			TopP:        o.config.TopP,
		}

		if chatRequest.Model == "" {
			chatRequest.Model = o.config.DefaultModel
		}

		// Call LLM
		response, err := o.llmClient.Chat(ctx, chatRequest)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrLLMUnavailable, err)
		}

		totalTokens += response.Usage.TotalTokens

		logging.LogDebugf("LLM response: role=%s content_len=%d tool_calls=%d",
			response.Message.Role, len(response.Message.Content), len(response.Message.ToolCalls))

		// Add assistant message to history
		messages = append(messages, response.Message)

		// Check if LLM wants to use tools
		if len(response.Message.ToolCalls) == 0 {
			// No tool calls, we're done
			logging.LogDebugf("Agent complete: iterations=%d tokens=%d", iteration, totalTokens)
			return &ChatResponse{
				Message:     response.Message,
				ToolsUsed:   toolExecutions,
				Iterations:  iteration,
				TotalTokens: totalTokens,
			}, nil
		}

		// Execute tool calls
		for _, toolCall := range response.Message.ToolCalls {
			execution, err := o.executeTool(ctx, request, toolCall)
			toolExecutions = append(toolExecutions, execution)

			if err != nil {
				// Add error as tool result
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					ToolCallID: toolCall.ID,
					Content:    fmt.Sprintf("Error: %v", err),
				})
			} else {
				// Add successful result as tool result
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					ToolCallID: toolCall.ID,
					Content:    execution.Result,
				})
			}
		}

		// Continue loop to get LLM's response to tool results
	}

	// Max iterations reached
	logging.LogWarningf(nil, "Agent max iterations reached: %d", o.config.MaxIterations)
	return &ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "I've reached my maximum thinking iterations. Please try rephrasing your question.",
		},
		ToolsUsed:   toolExecutions,
		Iterations:  iteration,
		TotalTokens: totalTokens,
		Error:       ErrMaxIterations,
	}, nil
}

// ExecuteStream runs the agent orchestration loop with streaming
func (o *Orchestrator) ExecuteStream(ctx context.Context, request ChatRequest) (<-chan StreamEvent, error) {
	eventChan := make(chan StreamEvent, 10)

	go func() {
		defer close(eventChan)

		// Build initial messages
		messages := o.buildMessages(request)

		// Get available tools (short-lived clients, no sessions yet)
		toolsWithServer, err := o.mcpManager.ListAllToolsForUser(ctx, request.UserID, request.BearerToken)
		if err != nil {
			eventChan <- StreamEvent{
				Type:  StreamEventTypeError,
				Error: errors.Wrap(err, "failed to get tools"),
				Done:  true,
			}
			return
		}

		// Convert to LLM format
		llmTools := make([]llm.Tool, 0, len(toolsWithServer))
		for _, t := range toolsWithServer {
			llmTools = append(llmTools, llm.ConvertMCPToolToLLMTool(t.Tool, t.ServerName))
		}

		iteration := 0

		for iteration < o.config.MaxIterations {
			iteration++

			// Build LLM request
			chatRequest := llm.ChatRequest{
				Model:       request.Model,
				Messages:    messages,
				Tools:       llmTools,
				Temperature: o.config.Temperature,
				MaxTokens:   o.config.MaxTokens,
				TopP:        o.config.TopP,
				Stream:      true,
			}

			if chatRequest.Model == "" {
				chatRequest.Model = o.config.DefaultModel
			}

			// Stream LLM response
			streamChan, err := o.llmClient.ChatStream(ctx, chatRequest)
			if err != nil {
				// Wrap with sentinel error for proper error detection
				wrapped := fmt.Errorf("%w: %v", ErrLLMUnavailable, err)
				logging.LogErrorf(wrapped, "Unable to start LLM streaming")
				eventChan <- StreamEvent{
					Type:  StreamEventTypeError,
					Error: wrapped,
					Done:  true,
				}
				return
			}

			// Collect full response
			var fullContent string
			var toolCalls []llm.ToolCall

			for chunk := range streamChan {
				if chunk.Error != nil {
					eventChan <- StreamEvent{
						Type:  StreamEventTypeError,
						Error: chunk.Error,
						Done:  true,
					}
					return
				}

				// Stream content
				if chunk.Delta.Content != "" {
					fullContent += chunk.Delta.Content
					eventChan <- StreamEvent{
						Type:    StreamEventTypeContent,
						Content: chunk.Delta.Content,
						Delta:   &chunk.Delta,
					}
				}

				// Collect tool calls
				if len(chunk.Delta.ToolCalls) > 0 {
					toolCalls = append(toolCalls, chunk.Delta.ToolCalls...)
				}

				if chunk.Done {
					break
				}
			}

			// Add assistant message to history
			assistantMsg := llm.Message{
				Role:      llm.RoleAssistant,
				Content:   fullContent,
				ToolCalls: toolCalls,
			}
			messages = append(messages, assistantMsg)

			// Check if done
			if len(toolCalls) == 0 {
				eventChan <- StreamEvent{
					Type: StreamEventTypeDone,
					Done: true,
				}
				return
			}

			// Execute tool calls
			for _, toolCall := range toolCalls {
				// Notify tool start (include parsed server and tool names for UI)
				{
					serverName, toolName := llm.ParseToolName(toolCall.Function.Name)
					eventChan <- StreamEvent{
						Type: StreamEventTypeToolStart,
						Tool: &ToolExecution{
							ServerName: serverName,
							ToolName:   toolName,
						},
					}
				}

				execution, err := o.executeTool(ctx, request, toolCall)

				// Notify tool complete
				eventChan <- StreamEvent{
					Type: StreamEventTypeToolComplete,
					Tool: &execution,
				}

				if err != nil {
					messages = append(messages, llm.Message{
						Role:       llm.RoleTool,
						ToolCallID: toolCall.ID,
						Content:    fmt.Sprintf("Error: %v", err),
					})
				} else {
					messages = append(messages, llm.Message{
						Role:       llm.RoleTool,
						ToolCallID: toolCall.ID,
						Content:    execution.Result,
					})
				}
			}

			// Continue loop
		}

		// Max iterations reached
		eventChan <- StreamEvent{
			Type:    StreamEventTypeError,
			Content: "Maximum iterations reached",
			Error:   ErrMaxIterations,
			Done:    true,
		}
	}()

	return eventChan, nil
}

// executeTool executes a single tool call
func (o *Orchestrator) executeTool(ctx context.Context, request ChatRequest, toolCall llm.ToolCall) (ToolExecution, error) {
	startTime := time.Now()

	execution := ToolExecution{
		ToolName: toolCall.Function.Name,
	}

	// Parse server and tool name
	serverName, toolName := llm.ParseToolName(toolCall.Function.Name)
	execution.ServerName = serverName
	// Store parsed tool name (without server prefix) for cleaner UI display
	execution.ToolName = toolName

	if serverName == "" {
		execution.Error = ErrInvalidToolName
		return execution, execution.Error
	}

	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		execution.Error = errors.Wrap(err, "failed to parse tool arguments")
		return execution, execution.Error
	}

	// Coerce/validate arguments to match the tool's input schema when possible
	// This helps when the model emits strings for numbers/booleans, etc.
	// Use user cache instead of session tools since session may not exist yet
	if toolsWithServer, err := o.mcpManager.ListAllToolsForUser(ctx, request.UserID, request.BearerToken); err == nil {
		var schema map[string]interface{}
		for _, tws := range toolsWithServer {
			if tws.ServerName == serverName && tws.Tool.Name == toolName {
				schema = tws.Tool.InputSchema
				break
			}
		}
		if schema != nil {
			logging.LogDebugf("Coercing args for %s.%s: before=%v schema=%v", serverName, toolName, args, schema)
			args = coerceArgumentsToSchema(schema, args)
			logging.LogDebugf("Coercing args for %s.%s: after=%v", serverName, toolName, args)
		}
	}

	execution.Arguments = args

	logging.LogDebugf("Executing tool: %s.%s with args: %v", serverName, toolName, args)

	// Ensure session exists for this server (just-in-time creation)
	mcpCfg := config.GetMCPConfig()
	var serverCfg config.MCPServerConfig
	for _, s := range mcpCfg.Servers {
		if s.Enabled && s.Name == serverName {
			serverCfg = s
			break
		}
	}
	if serverCfg.Name == "" {
		execution.Error = errors.Errorf("unknown or disabled server: %s", serverName)
		return execution, execution.Error
	}
	if _, err := o.mcpManager.GetOrCreateSession(ctx, request.ConversationID, serverCfg, request.BearerToken, request.UserID); err != nil {
		execution.Error = errors.Wrap(err, "failed to open MCP session")
		return execution, execution.Error
	}

	// Create timeout context
	toolCtx, cancel := context.WithTimeout(ctx, o.config.ToolExecutionTimeout)
	defer cancel()

	// Execute via MCP manager
	result, err := o.mcpManager.CallTool(toolCtx, request.ConversationID, serverName, toolName, args)
	execution.Duration = time.Since(startTime)

	if err != nil {
		execution.Error = err
		logging.LogErrorf(err, "Tool execution failed: %s.%s", serverName, toolName)
		return execution, err
	}

	// Convert result to string
	execution.Result = llm.ConvertMCPContentToString(result.Content)

	logging.LogDebugf("Tool execution complete: %s.%s duration=%v result_len=%d",
		serverName, toolName, execution.Duration, len(execution.Result))

	return execution, nil
}

// buildMessages constructs the initial message array
func (o *Orchestrator) buildMessages(request ChatRequest) []llm.Message {
	messages := make([]llm.Message, 0)

	// Add system prompt
	if o.config.SystemPrompt != "" {
		messages = append(messages, llm.Message{
			Role:    llm.RoleSystem,
			Content: o.config.SystemPrompt,
		})
	}

	// Add provided message history
	if len(request.Messages) > 0 {
		messages = append(messages, request.Messages...)
	}

	// Add current user message
	messages = append(messages, llm.Message{
		Role:    llm.RoleUser,
		Content: request.UserMessage,
	})

	return messages
}
