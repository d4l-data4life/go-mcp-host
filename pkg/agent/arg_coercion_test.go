package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoerceArgumentsToSchema_Primitives(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"latitude":      map[string]interface{}{"type": "number"},
			"longitude":     map[string]interface{}{"type": "number"},
			"days":          map[string]interface{}{"type": "integer"},
			"includeHourly": map[string]interface{}{"type": "boolean"},
		},
		"required": []interface{}{"latitude", "longitude"},
	}

	args := map[string]interface{}{
		"latitude":      "51.5074",
		"longitude":     "-0.1278",
		"days":          "3",
		"includeHourly": "true",
	}

	out := coerceArgumentsToSchema(schema, args)

	// latitude/longitude become float64
	assert.IsType(t, float64(0), out["latitude"])
	assert.IsType(t, float64(0), out["longitude"])
	// days is int64
	assert.IsType(t, int64(0), out["days"])
	// includeHourly is bool
	assert.IsType(t, true, out["includeHourly"])
}

func TestCoerceArgumentsToSchema_Nested(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"location": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"lat": map[string]interface{}{"type": "number"},
					"lon": map[string]interface{}{"type": "number"},
				},
			},
			"flags": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "integer"},
			},
		},
	}

	args := map[string]interface{}{
		"location": map[string]interface{}{
			"lat": "40.7",
			"lon": "-74.0",
		},
		"flags": []interface{}{"1", "2", 3.0},
	}

	out := coerceArgumentsToSchema(schema, args)

	loc := out["location"].(map[string]interface{})
	assert.IsType(t, float64(0), loc["lat"])
	assert.IsType(t, float64(0), loc["lon"])

	flags := out["flags"].([]interface{})
	assert.IsType(t, int64(0), flags[0])
	assert.IsType(t, int64(0), flags[1])
	assert.IsType(t, int64(0), flags[2])
}
