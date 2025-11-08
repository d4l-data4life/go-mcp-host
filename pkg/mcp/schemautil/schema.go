package schemautil

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

// ToolSchemaJSON returns the tool's input schema as raw JSON, preferring the
// server-provided raw schema when available.
func ToolSchemaJSON(tool mcp.Tool) json.RawMessage {
	if len(tool.RawInputSchema) > 0 {
		copied := make(json.RawMessage, len(tool.RawInputSchema))
		copy(copied, tool.RawInputSchema)
		return copied
	}

	if tool.InputSchema.Type == "" &&
		len(tool.InputSchema.Properties) == 0 &&
		len(tool.InputSchema.Required) == 0 &&
		len(tool.InputSchema.Defs) == 0 {
		return nil
	}

	data, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return nil
	}
	return data
}

// ToolSchemaMap unmarshals the schema into a generic map for consumers that
// require map[string]any (e.g. LLM tool definitions or argument coercion).
func ToolSchemaMap(tool mcp.Tool) map[string]interface{} {
	raw := ToolSchemaJSON(tool)
	if len(raw) == 0 {
		return defaultObjectSchema()
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return defaultObjectSchema()
	}

	if schema == nil {
		return defaultObjectSchema()
	}

	// Ensure object schemas always expose a properties map; OpenAI rejects
	// zero-argument tools that omit it.
	if schemaType, ok := schema["type"].(string); ok && schemaType == "object" {
		if props, hasProps := schema["properties"]; !hasProps || props == nil {
			schema["properties"] = map[string]interface{}{}
		}
	}

	return schema
}

func defaultObjectSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}
