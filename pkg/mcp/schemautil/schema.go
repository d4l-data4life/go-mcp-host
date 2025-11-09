package schemautil

import (
	"encoding/json"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolSchemaJSON returns the tool's input schema as raw JSON regardless of the shape
// provided by the MCP server.
func ToolSchemaJSON(tool *mcp.Tool) json.RawMessage {
	raw := normalizeToolSchema(tool)
	if len(raw) == 0 {
		return nil
	}

	copied := make(json.RawMessage, len(raw))
	copy(copied, raw)
	return copied
}

// ToolSchemaMap unmarshals the schema into a generic map for consumers that
// require map[string]any (e.g. LLM tool definitions or argument coercion).
func ToolSchemaMap(tool *mcp.Tool) map[string]interface{} {
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

func normalizeToolSchema(tool *mcp.Tool) []byte {
	if tool == nil || tool.InputSchema == nil {
		return nil
	}

	switch schema := tool.InputSchema.(type) {
	case json.RawMessage:
		if len(schema) == 0 {
			return nil
		}
		out := make([]byte, len(schema))
		copy(out, schema)
		return out
	case []byte:
		if len(schema) == 0 {
			return nil
		}
		out := make([]byte, len(schema))
		copy(out, schema)
		return out
	case string:
		if strings.TrimSpace(schema) == "" {
			return nil
		}
		return []byte(schema)
	default:
		data, err := json.Marshal(schema)
		if err != nil || len(data) == 0 {
			return nil
		}
		return data
	}
}

func defaultObjectSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}
