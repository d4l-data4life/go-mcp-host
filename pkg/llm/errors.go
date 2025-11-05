package llm

import "errors"

// Sentinel errors for LLM operations
var (
	// ErrConnectionFailed indicates the LLM connection failed
	ErrConnectionFailed = errors.New("LLM connection failed")

	// ErrRequestFailed indicates the LLM request failed
	ErrRequestFailed = errors.New("LLM request failed")
)

