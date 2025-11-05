package handlers

import (
	"errors"
	"fmt"
	"testing"

	"github.com/d4l-data4life/go-mcp-host/pkg/agent"
	"github.com/d4l-data4life/go-mcp-host/pkg/llm"
)

func TestShortenUserError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "Unexpected error",
		},
		{
			name:     "LLM unavailable sentinel error",
			err:      agent.ErrLLMUnavailable,
			expected: "LLM unavailable. Please check your model service.",
		},
		{
			name:     "wrapped LLM unavailable error",
			err:      fmt.Errorf("%w: connection refused", agent.ErrLLMUnavailable),
			expected: "LLM unavailable. Please check your model service.",
		},
		{
			name:     "max iterations error",
			err:      agent.ErrMaxIterations,
			expected: "Maximum iterations reached. Please try rephrasing your question.",
		},
		{
			name:     "invalid tool name error",
			err:      agent.ErrInvalidToolName,
			expected: "Invalid tool configuration. Please contact support.",
		},
		{
			name:     "tool execution failed error",
			err:      agent.ErrToolExecutionFailed,
			expected: "Tool execution failed. Please try again.",
		},
		{
			name:     "LLM connection failed error",
			err:      llm.ErrConnectionFailed,
			expected: "LLM connection failed. Please check your model service.",
		},
		{
			name:     "LLM request failed error",
			err:      llm.ErrRequestFailed,
			expected: "LLM request failed. Please try again.",
		},
		{
			name:     "unknown short error",
			err:      errors.New("something went wrong"),
			expected: "something went wrong",
		},
		{
			name: "unknown long error",
			err: errors.New(
				"this is a very long error message that exceeds the 140 character limit and should be truncated with an ellipsis at the end to make it more readable",
			),
			expected: "this is a very long error message that exceeds the 140 character limit and should be truncated with an ellipsis at the end to make it more r…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shortenUserError(tt.err)
			if result != tt.expected {
				t.Errorf("shortenUserError() = %q, want %q", result, tt.expected)
			}
		})
	}
}
