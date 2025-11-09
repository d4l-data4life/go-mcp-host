package agent

import (
	"context"
	"strings"

	"github.com/d4l-data4life/go-mcp-host/pkg/llm"
	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// GenerateChatTitle generates a short, descriptive title for a conversation based on the user's first message
func (a *Agent) GenerateChatTitle(ctx context.Context, userMessage string) string {
	logging.LogDebugf("Starting title generation for user message: %s", userMessage)

	// Create a prompt to generate a short title
	systemPrompt := `You are a title generator. Generate a short, descriptive title (max 6 words) for a chat conversation based on the user's first message.
Return ONLY the title text, no quotes, no explanations, no punctuation at the end.
Keep it concise and relevant to the main topic of the message.

Examples:
User: "What's the weather in San Francisco?"
Title: Weather in San Francisco

User: "Can you help me write a Python function to sort a list?"
Title: Python List Sorting Function

User: "I need to understand how quantum computing works"
Title: Understanding Quantum Computing

User: "What are the best practices for React hooks?"
Title: React Hooks Best Practices`

	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}

	// Use a simple, fast model for title generation
	// Set low temperature for consistent results
	req := llm.ChatRequest{
		Model:       a.config.DefaultModel,
		Messages:    messages,
		Temperature: float64Ptr(0.3),
		MaxTokens:   intPtr(50), // Short titles only
		Stream:      false,
	}

	logging.LogDebugf("Sending title generation request to LLM model: %s", a.config.DefaultModel)
	response, err := a.llmClient.Chat(ctx, req)
	if err != nil {
		logging.LogErrorf(err, "Failed to generate chat title - LLM request failed")
		return "" // Return empty to keep default
	}

	// Clean up the title
	rawTitle := response.Message.Content
	logging.LogDebugf("Raw title from LLM: %q", rawTitle)

	title := strings.TrimSpace(rawTitle)

	// Remove quotes if present
	title = strings.Trim(title, `"'`)
	logging.LogDebugf("Title after quote trimming: %q", title)

	// Truncate if too long
	if len(title) > 60 {
		logging.LogDebugf("Title too long (%d chars), truncating to 60 chars", len(title))
		title = title[:57] + "..."
	}

	// If title is empty or too short, return empty to keep default
	if len(title) < 3 {
		logging.LogDebugf("Title too short (%d chars), keeping default title", len(title))
		return ""
	}

	logging.LogDebugf("Successfully generated chat title: %q", title)
	return title
}

func float64Ptr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
