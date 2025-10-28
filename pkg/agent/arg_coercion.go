package agent

import (
	"encoding/json"
	"strconv"
	"strings"
)

// coerceArgumentsToSchema attempts to coerce primitive argument types to match the JSON Schema
// Only a pragmatic subset is implemented (number, integer, boolean, nested objects and arrays)
func coerceArgumentsToSchema(schema map[string]interface{}, args map[string]interface{}) map[string]interface{} {
	if args == nil || schema == nil {
		return args
	}

	// Expect object schema with properties
	props, _ := schema["properties"].(map[string]interface{})
	if props == nil {
		return args
	}

	coerced := make(map[string]interface{}, len(args))
	for key, val := range args {
		if rawPropSchema, ok := props[key]; ok {
			if propSchema, ok := rawPropSchema.(map[string]interface{}); ok {
				if newVal, changed := coerceValue(propSchema, val); changed {
					coerced[key] = newVal
					continue
				}
			}
		}
		// Default: keep value as-is
		coerced[key] = val
	}
	return coerced
}

// coerceValue coerces a value based on the provided (sub)schema
func coerceValue(schema map[string]interface{}, val interface{}) (interface{}, bool) {
	// Extract type which could be string or array (union)
	var types []string
	switch t := schema["type"].(type) {
	case string:
		types = []string{t}
	case []interface{}:
		for _, v := range t {
			if s, ok := v.(string); ok {
				types = append(types, s)
			}
		}
	}

	// If object, recurse into properties
	if containsType(types, "object") {
		if obj, ok := val.(map[string]interface{}); ok {
			return coerceArgumentsToSchema(schema, obj), true
		}
		// Try to parse from JSON string if provided
		if str, ok := val.(string); ok {
			var parsed map[string]interface{}
			if json.Unmarshal([]byte(str), &parsed) == nil {
				return coerceArgumentsToSchema(schema, parsed), true
			}
		}
		return val, false
	}

	// Arrays: attempt to coerce items
	if containsType(types, "array") {
		itemsSchema, _ := schema["items"].(map[string]interface{})
		if itemsSchema == nil {
			return val, false
		}
		arr, ok := val.([]interface{})
		if !ok {
			// Attempt to parse JSON array from string
			if str, isStr := val.(string); isStr {
				var parsed []interface{}
				if json.Unmarshal([]byte(str), &parsed) == nil {
					arr = parsed
				} else {
					return val, false
				}
			} else {
				return val, false
			}
		}
		changed := false
		for i, item := range arr {
			if nv, ch := coerceValue(itemsSchema, item); ch {
				arr[i] = nv
				changed = true
			}
		}
		return arr, changed
	}

	// Numbers
	if containsType(types, "number") {
		switch v := val.(type) {
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				return f, true
			}
		}
		return val, false
	}

	// Integers
	if containsType(types, "integer") {
		switch v := val.(type) {
		case string:
			if i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
				return i, true
			}
		case float64:
			// If model returned 3.0 for integer, coerce down
			if float64(int64(v)) == v {
				return int64(v), true
			}
		}
		return val, false
	}

	// Booleans
	if containsType(types, "boolean") {
		switch v := val.(type) {
		case string:
			s := strings.TrimSpace(strings.ToLower(v))
			if s == "true" {
				return true, true
			}
			if s == "false" {
				return false, true
			}
		}
		return val, false
	}

	return val, false
}

func containsType(types []string, t string) bool {
	for _, x := range types {
		if x == t {
			return true
		}
	}
	return false
}
