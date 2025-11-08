package agent

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/d4l-data4life/go-mcp-host/pkg/llm"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/manager"
)

// ContextManager manages context selection and pruning for the agent
type ContextManager struct {
	mcpManager       *manager.Manager
	maxContextTokens int
}

// NewContextManager creates a new context manager
func NewContextManager(mcpManager *manager.Manager, maxContextTokens int) *ContextManager {
	return &ContextManager{
		mcpManager:       mcpManager,
		maxContextTokens: maxContextTokens,
	}
}

// SelectRelevantResources finds resources relevant to the user's query
func (cm *ContextManager) SelectRelevantResources(
	ctx context.Context,
	userID uuid.UUID,
	bearerToken string,
	query string,
) ([]ResourceWithRelevance, error) {
	// Get all available resources
	resourcesWithServer, err := cm.mcpManager.ListAllResourcesForUser(ctx, userID, bearerToken)
	if err != nil {
		return nil, err
	}

	// Score each resource for relevance
	results := make([]ResourceWithRelevance, 0, len(resourcesWithServer))
	for _, r := range resourcesWithServer {
		score := cm.scoreResourceRelevance(r, query)
		if score > 0 {
			results = append(results, ResourceWithRelevance{
				Resource:   r,
				Relevance:  score,
				ServerName: r.ServerName,
			})
		}
	}

	// Sort by relevance (highest first)
	sortByRelevance(results)

	return results, nil
}

// ReadRelevantResources reads the content of relevant resources
func (cm *ContextManager) ReadRelevantResources(
	ctx context.Context,
	conversationID uuid.UUID,
	resources []ResourceWithRelevance,
	maxResources int,
) ([]ResourceContent, error) {
	if maxResources > len(resources) {
		maxResources = len(resources)
	}

	contents := make([]ResourceContent, 0, maxResources)

	for i := 0; i < maxResources; i++ {
		r := resources[i]

		// Read resource via MCP
		result, err := cm.mcpManager.ReadResource(ctx, conversationID, r.ServerName, r.Resource.Resource.URI)
		if err != nil {
			// Log error but continue with other resources
			continue
		}

		content := resourceContentToString(result.Contents)

		contents = append(contents, ResourceContent{
			URI:       r.Resource.Resource.URI,
			Name:      r.Resource.Resource.Name,
			Content:   content,
			Relevance: r.Relevance,
		})
	}

	return contents, nil
}

// PruneMessages removes old messages to fit within token limit
func (cm *ContextManager) PruneMessages(messages []llm.Message, maxTokens int) []llm.Message {
	// Simple pruning: keep system message and recent messages
	if len(messages) <= 2 {
		return messages
	}

	// Estimate tokens (rough approximation: 1 token ≈ 4 chars)
	estimateTokens := func(msg llm.Message) int {
		return len(msg.Content) / 4
	}

	totalTokens := 0
	for _, msg := range messages {
		totalTokens += estimateTokens(msg)
	}

	if totalTokens <= maxTokens {
		return messages
	}

	// Keep system message
	pruned := []llm.Message{messages[0]}
	currentTokens := estimateTokens(messages[0])

	// Add recent messages from the end
	for i := len(messages) - 1; i > 0; i-- {
		msgTokens := estimateTokens(messages[i])
		if currentTokens+msgTokens > maxTokens {
			break
		}
		pruned = append([]llm.Message{messages[i]}, pruned...)
		currentTokens += msgTokens
	}

	return pruned
}

// scoreResourceRelevance scores how relevant a resource is to the query
func (cm *ContextManager) scoreResourceRelevance(resource manager.ResourceWithServer, query string) float64 {
	score := 0.0
	queryLower := strings.ToLower(query)

	// Check name
	if resource.Resource.Name != "" {
		nameLower := strings.ToLower(resource.Resource.Name)
		if strings.Contains(queryLower, nameLower) || strings.Contains(nameLower, queryLower) {
			score += 0.5
		}
	}

	// Check description
	if resource.Resource.Description != "" {
		descLower := strings.ToLower(resource.Resource.Description)
		for _, word := range strings.Fields(queryLower) {
			if strings.Contains(descLower, word) {
				score += 0.1
			}
		}
	}

	// Check URI
	if resource.Resource.URI != "" {
		uriLower := strings.ToLower(resource.Resource.URI)
		for _, word := range strings.Fields(queryLower) {
			if strings.Contains(uriLower, word) {
				score += 0.05
			}
		}
	}

	return score
}

// sortByRelevance sorts resources by relevance score (descending)
func sortByRelevance(resources []ResourceWithRelevance) {
	// Simple bubble sort (good enough for small lists)
	n := len(resources)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if resources[j].Relevance < resources[j+1].Relevance {
				resources[j], resources[j+1] = resources[j+1], resources[j]
			}
		}
	}
}

// ResourceWithRelevance represents a resource with its relevance score
type ResourceWithRelevance struct {
	Resource   manager.ResourceWithServer
	ServerName string
	Relevance  float64
}

// ResourceContent represents the content of a resource
type ResourceContent struct {
	URI       string
	Name      string
	Content   string
	Relevance float64
}

func resourceContentToString(contents []mcp.ResourceContents) string {
	if len(contents) == 0 {
		return ""
	}

	for _, c := range contents {
		switch rc := c.(type) {
		case mcp.TextResourceContents:
			if rc.Text != "" {
				return rc.Text
			}
		case *mcp.TextResourceContents:
			if rc.Text != "" {
				return rc.Text
			}
		case mcp.BlobResourceContents:
			if rc.Blob != "" {
				return "[Binary content]"
			}
		case *mcp.BlobResourceContents:
			if rc.Blob != "" {
				return "[Binary content]"
			}
		}
	}

	return ""
}
