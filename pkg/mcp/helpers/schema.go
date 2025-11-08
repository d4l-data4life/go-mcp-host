package helpers

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

// ToolInputSchemaToMap converts the structured or raw MCP tool schema into a generic map.
// MCP servers can provide either the structured ToolInputSchema or a raw JSON schema, so we
// normalize to a map[string]interface{} for downstream consumers like the LLM tooling layer.
func ToolInputSchemaToMap(tool mcp.Tool) map[string]interface{} {
	var (
		rawSchema json.RawMessage
		err       error
	)

	switch {
	case len(tool.RawInputSchema) > 0:
		rawSchema = tool.RawInputSchema
	case tool.InputSchema.Type != "":
		rawSchema, err = json.Marshal(tool.InputSchema)
		if err != nil {
			return map[string]interface{}{}
		}
	default:
		return map[string]interface{}{}
	}

	if len(rawSchema) == 0 {
		return map[string]interface{}{}
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(rawSchema, &schema); err != nil {
		return map[string]interface{}{}
	}

	if schema == nil {
		schema = map[string]interface{}{}
	}

	return schema
}
