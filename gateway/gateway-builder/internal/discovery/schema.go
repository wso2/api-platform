package discovery

import "log/slog"

// ExtractDefaultValues extracts default values from a JSON schema structure.
// It processes the "properties" object and extracts either "default" or "wso2/defaultValue"
// with precedence given to "wso2/defaultValue" when both exist.
//
// Input schema format (from initParameters in policy-definition.yaml):
//
//	{
//	  "type": "object",
//	  "properties": {
//	    "propName": {
//	      "type": "string",
//	      "default": "value1",
//	      "wso2/defaultValue": "$config(Path.To.Config)"
//	    }
//	  }
//	}
//
// Returns: map[string]interface{} with only the default values
//
//	{"propName": "$config(Path.To.Config)"}
//
// TODO: (renuka) handle nested objects
func ExtractDefaultValues(schema map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Handle nil or empty schema
	if schema == nil {
		slog.Debug("Schema is nil, returning empty map", "phase", "discovery")
		return result
	}

	// Extract properties object
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		slog.Debug("No properties found in schema", "phase", "discovery")
		return result
	}

	slog.Debug("Extracting defaults from schema",
		"propertyCount", len(properties),
		"phase", "discovery")

	// Iterate through each property
	for propName, propDef := range properties {
		propDefMap, ok := propDef.(map[string]interface{})
		if !ok {
			slog.Debug("Property definition is not a map, skipping",
				"property", propName,
				"phase", "discovery")
			continue
		}

		// Check for wso2/defaultValue first (higher precedence)
		if wso2Default, exists := propDefMap["wso2/defaultValue"]; exists {
			result[propName] = wso2Default
			slog.Debug("Extracted wso2/defaultValue",
				"property", propName,
				"value", wso2Default,
				"phase", "discovery")
			continue
		}

		// Fallback to standard default
		if defaultValue, exists := propDefMap["default"]; exists {
			result[propName] = defaultValue
			slog.Debug("Extracted default",
				"property", propName,
				"value", defaultValue,
				"phase", "discovery")
		}
	}

	slog.Debug("Extraction complete",
		"extractedCount", len(result),
		"phase", "discovery")

	return result
}
