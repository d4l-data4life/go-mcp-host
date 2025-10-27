package llm

import (
	"github.com/weese/go-mcp-host/pkg/mcp/protocol"
)

// ConvertMCPToolToLLMTool converts an MCP tool to LLM function format
func ConvertMCPToolToLLMTool(mcpTool protocol.Tool, serverName string) Tool {
	return Tool{
		Type: ToolTypeFunction,
		Function: ToolFunction{
			Name:        serverName + "_" + mcpTool.Name, // Prefix with server name to avoid conflicts
			Description: mcpTool.Description,
			Parameters:  mcpTool.InputSchema,
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

// ParseToolName extracts server name and tool name from a combined tool name
// Format: "serverName_toolName"
func ParseToolName(combinedName string) (serverName, toolName string) {
	// Find the first underscore
	for i := 0; i < len(combinedName); i++ {
		if combinedName[i] == '_' {
			return combinedName[:i], combinedName[i+1:]
		}
	}
	// If no underscore found, return empty server name
	return "", combinedName
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

