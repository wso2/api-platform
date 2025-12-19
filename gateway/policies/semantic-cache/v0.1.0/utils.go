package semanticcache

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// floatsToBytes converts float32 slice to byte array for storing in Redis
// Uses little-endian byte order as required by RediSearch
func floatsToBytes(fs []float32) []byte {
	buf := make([]byte, len(fs)*4)
	for i, f := range fs {
		u := math.Float32bits(f)
		binary.LittleEndian.PutUint32(buf[i*4:], u)
	}
	return buf
}

// ExtractStringValueFromJSONPath extracts a value from JSON using JSONPath
func ExtractStringValueFromJSONPath(payload []byte, jsonPath string) (string, error) {
	if jsonPath == "" {
		return string(payload), nil
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(payload, &jsonData); err != nil {
		return "", fmt.Errorf("error unmarshaling JSON: %w", err)
	}

	value, err := extractValueFromJSONPath(jsonData, jsonPath)
	if err != nil {
		return "", err
	}

	// Convert to string
	switch v := value.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// extractValueFromJSONPath extracts a value from a nested JSON structure based on a JSON path
func extractValueFromJSONPath(data map[string]interface{}, jsonPath string) (interface{}, error) {
	keys := strings.Split(jsonPath, ".")
	if len(keys) > 0 && keys[0] == "$" {
		keys = keys[1:]
	}

	return extractRecursive(data, keys)
}

func extractRecursive(current interface{}, keys []string) (interface{}, error) {
	if len(keys) == 0 {
		return current, nil
	}

	key := keys[0]
	remaining := keys[1:]

	// Handle array indexing (e.g., "items[0]")
	if idx := strings.Index(key, "["); idx != -1 {
		arrayName := key[:idx]
		idxStr := key[idx+1 : len(key)-1]
		idxVal, err := strconv.Atoi(idxStr)
		if err != nil {
			return nil, fmt.Errorf("invalid array index: %s", idxStr)
		}

		if node, ok := current.(map[string]interface{}); ok {
			if arrVal, exists := node[arrayName]; exists {
				if arr, ok := arrVal.([]interface{}); ok {
					if idxVal < 0 {
						idxVal = len(arr) + idxVal
					}
					if idxVal < 0 || idxVal >= len(arr) {
						return nil, fmt.Errorf("array index out of range: %d", idxVal)
					}
					return extractRecursive(arr[idxVal], remaining)
				}
				return nil, fmt.Errorf("not an array: %s", arrayName)
			}
			return nil, fmt.Errorf("key not found: %s", arrayName)
		}
		return nil, fmt.Errorf("invalid structure for key: %s", arrayName)
	}

	// Handle regular key access
	if node, ok := current.(map[string]interface{}); ok {
		if val, exists := node[key]; exists {
			return extractRecursive(val, remaining)
		}
		return nil, fmt.Errorf("key not found: %s", key)
	}

	return nil, fmt.Errorf("cannot access key %s on non-map value", key)
}
