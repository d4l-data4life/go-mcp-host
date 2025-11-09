package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

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

	llmTools, toolLookup, err := o.prepareToolContext(ctx, request)
	if err != nil {
		return nil, err
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

		logLLMRequest("chat", chatRequest)

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
			execution, content := o.handleToolCall(ctx, request, toolCall, toolLookup)
			toolExecutions = append(toolExecutions, execution)
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: toolCall.ID,
				Content:    content,
			})
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

		llmTools, toolLookup, err := o.prepareToolContext(ctx, request)
		if err != nil {
			eventChan <- StreamEvent{
				Type:  StreamEventTypeError,
				Error: err,
				Done:  true,
			}
			return
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

			logLLMRequest("chat-stream", chatRequest)

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
			var contentBuilder strings.Builder
			var assistantMsg llm.Message
			assistantMsgSet := false

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
					contentBuilder.WriteString(chunk.Delta.Content)
					eventChan <- StreamEvent{
						Type:    StreamEventTypeContent,
						Content: chunk.Delta.Content,
						Delta:   &chunk.Delta,
					}
				}

				if chunk.Message != nil {
					assistantMsg = *chunk.Message
					assistantMsgSet = true
				}

				if chunk.Done {
					break
				}
			}

			finalContent := contentBuilder.String()

			// Add assistant message to history
			if !assistantMsgSet {
				assistantMsg = llm.Message{
					Role:    llm.RoleAssistant,
					Content: finalContent,
				}
			} else if assistantMsg.Content == "" {
				assistantMsg.Content = finalContent
			}
			messages = append(messages, assistantMsg)
			toolCalls := assistantMsg.ToolCalls

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
				binding, ok := toolLookup[toolCall.Function.Name]
				if !ok {
					invalid := ToolExecution{
						ToolName: toolCall.Function.Name,
						Error:    ErrInvalidToolName,
					}
					eventChan <- StreamEvent{
						Type: StreamEventTypeToolStart,
						Tool: &invalid,
					}
					eventChan <- StreamEvent{
						Type: StreamEventTypeToolComplete,
						Tool: &invalid,
					}
					messages = append(messages, llm.Message{
						Role:       llm.RoleTool,
						ToolCallID: toolCall.ID,
						Content:    fmt.Sprintf("Error: %v", ErrInvalidToolName),
					})
					continue
				}

				start := ToolExecution{
					ServerName: binding.ServerName,
					ToolName:   binding.Tool.Name,
				}
				eventChan <- StreamEvent{
					Type: StreamEventTypeToolStart,
					Tool: &start,
				}

				execution, err := o.executeTool(ctx, request, toolCall, binding)

				completed := execution
				eventChan <- StreamEvent{
					Type: StreamEventTypeToolComplete,
					Tool: &completed,
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

func logLLMRequest(label string, req llm.ChatRequest) {
	payload, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		logging.LogDebugf("LLM request (%s) marshal error: %v", label, err)
		logging.LogDebugf("LLM request (%s) summary: model=%s messages=%d tools=%d stream=%v",
			label, req.Model, len(req.Messages), len(req.Tools), req.Stream)
		return
	}
	logging.LogDebugf("LLM request (%s):\n%s", label, string(payload))
}

// prepareToolContext fetches available tools and builds both LLM tool definitions and a reverse lookup map.
func (o *Orchestrator) prepareToolContext(ctx context.Context, request ChatRequest) ([]llm.Tool, map[string]manager.ToolWithServer, error) {
	toolsWithServer, err := o.mcpManager.ListAllToolsForUser(ctx, request.UserID, request.BearerToken)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get tools")
	}

	llmTools := make([]llm.Tool, 0, len(toolsWithServer))
	lookup := make(map[string]manager.ToolWithServer, len(toolsWithServer))

	for _, t := range toolsWithServer {
		llmTool := llm.ConvertMCPToolToLLMTool(t.Tool, t.ServerName)
		llmTools = append(llmTools, llmTool)
		lookup[llmTool.Function.Name] = t
	}

	return llmTools, lookup, nil
}

// handleToolCall executes a tool and returns its execution record plus the message content to append.
func (o *Orchestrator) handleToolCall(
	ctx context.Context,
	request ChatRequest,
	toolCall llm.ToolCall,
	toolLookup map[string]manager.ToolWithServer,
) (ToolExecution, string) {
	binding, ok := toolLookup[toolCall.Function.Name]
	if !ok {
		execution := ToolExecution{
			ToolName: toolCall.Function.Name,
			Error:    ErrInvalidToolName,
		}
		logging.LogWarningf(ErrInvalidToolName, "Unknown tool requested by LLM: %s", toolCall.Function.Name)
		return execution, fmt.Sprintf("Error: %v", ErrInvalidToolName)
	}

	execution, err := o.executeTool(ctx, request, toolCall, binding)
	if err != nil {
		return execution, fmt.Sprintf("Error: %v", err)
	}
	return execution, execution.Result
}

// executeTool executes a single tool call
func (o *Orchestrator) executeTool(
	ctx context.Context,
	request ChatRequest,
	toolCall llm.ToolCall,
	binding manager.ToolWithServer,
) (ToolExecution, error) {
	startTime := time.Now()

	execution := ToolExecution{
		ServerName: binding.ServerName,
		ToolName:   binding.Tool.Name,
	}

	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		execution.Error = errors.Wrap(err, "failed to parse tool arguments")
		return execution, execution.Error
	}

	// Coerce/validate arguments to match the tool's input schema when possible
	// This helps when the model emits strings for numbers/booleans, etc.
	if schema := binding.Tool.InputSchema; schema != nil {
		logging.LogDebugf("Coercing args for %s.%s: before=%v schema=%v", binding.ServerName, binding.Tool.Name, args, schema)
		args = coerceArgumentsToSchema(schema, args)
		logging.LogDebugf("Coercing args for %s.%s: after=%v", binding.ServerName, binding.Tool.Name, args)
	}

	execution.Arguments = args

	logging.LogDebugf("Executing tool: %s.%s with args: %v", binding.ServerName, binding.Tool.Name, args)

	// Ensure session exists for this server (just-in-time creation)
	serverCfg, ok := o.mcpManager.GetServerConfig(binding.ServerName)
	if !ok {
		execution.Error = errors.Errorf("unknown or disabled server: %s", binding.ServerName)
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
	result, err := o.mcpManager.CallTool(toolCtx, request.ConversationID, binding.ServerName, binding.Tool.Name, args)
	execution.Duration = time.Since(startTime)

	if err != nil {
		execution.Error = err
		logging.LogErrorf(err, "Tool execution failed: %s.%s", binding.ServerName, binding.Tool.Name)
		return execution, err
	}

	// Convert result to string
	execution.Result = llm.ConvertMCPContentToString(result.Content)

	logging.LogDebugf("Tool execution complete: %s.%s duration=%v result_len=%d",
		binding.ServerName, binding.Tool.Name, execution.Duration, len(execution.Result))

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
