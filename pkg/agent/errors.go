package agent

import "errors"

// Sentinel errors for agent operations
var (
	// ErrLLMUnavailable indicates the LLM service is unreachable or not responding
	ErrLLMUnavailable = errors.New("LLM service unavailable")

	// ErrMaxIterations indicates the agent reached maximum iterations
	ErrMaxIterations = errors.New("max iterations reached")

	// ErrInvalidToolName indicates a tool name format is invalid
	ErrInvalidToolName = errors.New("invalid tool name format")

	// ErrToolExecutionFailed indicates a tool execution failed
	ErrToolExecutionFailed = errors.New("tool execution failed")
)

