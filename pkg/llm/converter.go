package llm

import (
	"encoding/json"
	"fmt"

	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/protocol"
)

const toolNameSeparator = "__"

// QualifiedToolName returns the canonical tool identifier (<server>__<tool>)
func QualifiedToolName(serverName, toolName string) string {
	return fmt.Sprintf("%s%s%s", serverName, toolNameSeparator, toolName)
}

// ConvertMCPToolToLLMTool converts an MCP tool to LLM function format
func ConvertMCPToolToLLMTool(mcpTool protocol.Tool, serverName string) Tool {
	// Clone schema (don't add "strict" as it confuses some models)
	parameters := cloneMap(mcpTool.InputSchema)

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
func ConvertMCPToolsToLLMTools(mcpTools []protocol.Tool, serverName string) []Tool {
	llmTools := make([]Tool, len(mcpTools))
	for i, mcpTool := range mcpTools {
		llmTools[i] = ConvertMCPToolToLLMTool(mcpTool, serverName)
	}
	return llmTools
}

// ConvertMCPContentToString converts MCP content array to a single string
func ConvertMCPContentToString(contents []protocol.Content) string {
	if len(contents) == 0 {
		return ""
	}

	// For now, just concatenate text content
	result := ""
	for i, content := range contents {
		if i > 0 {
			result += "\n"
		}
		result += content.Text
	}
	return result
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
