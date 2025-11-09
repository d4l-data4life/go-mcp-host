package helpers

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	schemautil "github.com/d4l-data4life/go-mcp-host/pkg/mcp/schemautil"
)

// ToolInputSchemaToMap converts the structured or raw MCP tool schema into a generic map.
// MCP servers can provide either the structured ToolInputSchema or a raw JSON schema, so we
// normalize to a map[string]interface{} for downstream consumers like the LLM tooling layer.
func ToolInputSchemaToMap(tool *mcp.Tool) map[string]interface{} {
	return schemautil.ToolSchemaMap(tool)
}
