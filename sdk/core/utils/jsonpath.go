package utils

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var arrayIndexRegex = regexp.MustCompile(`^([a-zA-Z0-9_]+)\[(-?\d+)\]$`)

// ExtractStringValueFromJsonpath extracts a value from a nested JSON structure based on a JSON path.
func ExtractStringValueFromJsonpath(payload []byte, jsonpath string) (string, error) {
	if jsonpath == "" {
		return string(payload), nil
	}
	var jsonData map[string]interface{}
	if err := json.Unmarshal(payload, &jsonData); err != nil {
		return "", err
	}
	value, err := ExtractValueFromJsonpath(jsonData, jsonpath)
	if err != nil {
		return "", err
	}
	// Convert to string if possible
	switch v := value.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(v), nil
	default:
		return "", errors.New("value at JSONPath is not a string or number")
	}
}

// extractValueFromJsonpath extracts a value from a nested JSON structure based on a JSON path.
func ExtractValueFromJsonpath(data map[string]interface{}, jsonpath string) (interface{}, error) {
	keys := strings.Split(jsonpath, ".")
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

	if key == "*" {
		var results []interface{}
		switch node := current.(type) {
		case map[string]interface{}:
			for _, v := range node {
				res, err := extractRecursive(v, remaining)
				if err == nil {
					results = append(results, res)
				}
			}
		case []interface{}:
			for _, v := range node {
				res, err := extractRecursive(v, remaining)
				if err == nil {
					results = append(results, res)
				}
			}
		default:
			return nil, errors.New("wildcard used on non-iterable node")
		}
		return results, nil
	}

	if matches := arrayIndexRegex.FindStringSubmatch(key); len(matches) == 3 {
		arrayName := matches[1]
		idxStr := matches[2]
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			return nil, errors.New("invalid array index: " + idxStr)
		}
		if node, ok := current.(map[string]interface{}); ok {
			if arrVal, exists := node[arrayName]; exists {
				if arr, ok := arrVal.([]interface{}); ok {
					if idx < 0 {
						idx = len(arr) + idx
					}
					if idx < 0 || idx >= len(arr) {
						return nil, errors.New("array index out of range: " + idxStr)
					}
					return extractRecursive(arr[idx], remaining)
				}
				return nil, errors.New("not an array: " + arrayName)
			}
			return nil, errors.New("key not found: " + arrayName)
		}
		return nil, errors.New("invalid structure for key: " + arrayName)
	}

	if node, ok := current.(map[string]interface{}); ok {
		if val, exists := node[key]; exists {
			return extractRecursive(val, remaining)
		}
		return nil, errors.New("key not found: " + key)
	}
	return nil, errors.New("invalid structure for key: " + key)
}

// SetValueAtJSONPath sets a value at the specified JSONPath in the given JSON object
func SetValueAtJSONPath(jsonData map[string]interface{}, jsonPath, value string) error {
	// Remove the leading "$." if present
	path := strings.TrimPrefix(jsonPath, "$.")
	if path == "" {
		return errors.New("invalid empty path")
	}

	// Split the path into components
	pathComponents := strings.Split(path, ".")

	// Navigate to the parent object/array
	current := interface{}(jsonData)
	for i := 0; i < len(pathComponents)-1; i++ {
		key := pathComponents[i]

		// Check if this key contains array indexing
		if matches := arrayIndexRegex.FindStringSubmatch(key); len(matches) == 3 {
			arrayName := matches[1]
			idxStr := matches[2]
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return errors.New("invalid array index: " + idxStr)
			}

			if node, ok := current.(map[string]interface{}); ok {
				if arrVal, exists := node[arrayName]; exists {
					if arr, ok := arrVal.([]interface{}); ok {
						if idx < 0 {
							idx = len(arr) + idx
						}
						if idx < 0 || idx >= len(arr) {
							return errors.New("array index out of range: " + idxStr)
						}
						current = arr[idx]
					} else {
						return errors.New("not an array: " + arrayName)
					}
				} else {
					return errors.New("key not found: " + arrayName)
				}
			} else {
				return errors.New("invalid structure for key: " + arrayName)
			}
		} else {
			// Regular object key
			if node, ok := current.(map[string]interface{}); ok {
				if val, exists := node[key]; exists {
					current = val
				} else {
					return errors.New("key not found: " + key)
				}
			} else {
				return errors.New("invalid structure for key: " + key)
			}
		}
	}

	// Handle the final key (could be array index or object key)
	finalKey := pathComponents[len(pathComponents)-1]

	// Check if the final key contains array indexing
	if matches := arrayIndexRegex.FindStringSubmatch(finalKey); len(matches) == 3 {
		arrayName := matches[1]
		idxStr := matches[2]
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			return errors.New("invalid array index: " + idxStr)
		}

		if node, ok := current.(map[string]interface{}); ok {
			if arrVal, exists := node[arrayName]; exists {
				if arr, ok := arrVal.([]interface{}); ok {
					if idx < 0 {
						idx = len(arr) + idx
					}
					if idx < 0 || idx >= len(arr) {
						return errors.New("array index out of range: " + idxStr)
					}
					arr[idx] = value
				} else {
					return errors.New("not an array: " + arrayName)
				}
			} else {
				return errors.New("key not found: " + arrayName)
			}
		} else {
			return errors.New("invalid structure for key: " + arrayName)
		}
	} else {
		// Regular object key
		if node, ok := current.(map[string]interface{}); ok {
			node[finalKey] = value
		} else {
			return errors.New("invalid structure for final key: " + finalKey)
		}
	}

	return nil
}
