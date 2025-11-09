package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	schemautil "github.com/d4l-data4life/go-mcp-host/pkg/mcp/schemautil"
)

const toolNameSeparator = "__"

// QualifiedToolName returns the canonical tool identifier (<server>__<tool>)
func QualifiedToolName(serverName, toolName string) string {
	return fmt.Sprintf("%s%s%s", serverName, toolNameSeparator, toolName)
}

// ConvertMCPToolToLLMTool converts an MCP tool to LLM function format
func ConvertMCPToolToLLMTool(mcpTool *mcp.Tool, serverName string) Tool {
	if mcpTool == nil {
		return Tool{
			Type: ToolTypeFunction,
			Function: ToolFunction{
				Name:       QualifiedToolName(serverName, "unknown"),
				Parameters: map[string]interface{}{},
			},
		}
	}

	// Clone schema (don't add "strict" as it confuses some models)
	parameters := cloneMap(schemautil.ToolSchemaMap(mcpTool))

	return Tool{
		Type: ToolTypeFunction,
		Function: ToolFunction{
			Name:        QualifiedToolName(serverName, mcpTool.Name),
			Description: mcpTool.Description,
			Parameters:  parameters,
		},
	}
}

// ConvertMCPToolsToLLMTools converts multiple MCP tools to LLM format
func ConvertMCPToolsToLLMTools(mcpTools []*mcp.Tool, serverName string) []Tool {
	llmTools := make([]Tool, len(mcpTools))
	for i, mcpTool := range mcpTools {
		llmTools[i] = ConvertMCPToolToLLMTool(mcpTool, serverName)
	}
	return llmTools
}

// ConvertMCPContentToString converts MCP content array to a single string
func ConvertMCPContentToString(contents []mcp.Content) string {
	if len(contents) == 0 {
		return ""
	}

	var builder strings.Builder

	appendLine := func(text string) {
		if text == "" {
			return
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(text)
	}

	for _, content := range contents {
		switch c := content.(type) {
		case *mcp.TextContent:
			appendLine(c.Text)
		case *mcp.EmbeddedResource:
			if c.Resource == nil {
				continue
			}
			if c.Resource.Text != "" {
				appendLine(c.Resource.Text)
				continue
			}
			if len(c.Resource.Blob) > 0 {
				appendLine("[Binary resource]")
			}
		case *mcp.ResourceLink:
			appendLine(fmt.Sprintf("[Resource link] %s", c.URI))
		}
	}

	return builder.String()
}

// cloneMap performs a deep copy of a generic map to avoid mutating original schemas
func cloneMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	// Use JSON round-trip for a safe deep copy of arbitrary structures
	b, _ := json.Marshal(src)
	var dst map[string]interface{}
	_ = json.Unmarshal(b, &dst)
	if dst == nil {
		dst = make(map[string]interface{})
	}
	return dst
}
