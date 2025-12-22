/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package registry

import (
	"fmt"
	"log/slog"
	"regexp"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// ConfigResolver resolves ${...} CEL expressions from a configuration map
type ConfigResolver struct {
	config map[string]interface{}
	env    *cel.Env
}

// NewConfigResolver creates a new config resolver with CEL environment
func NewConfigResolver(config map[string]interface{}) (*ConfigResolver, error) {
	// Create CEL environment with config variable
	env, err := cel.NewEnv(
		cel.Variable("config", cel.DynType),
	)
	if err != nil {
		slog.Error("Failed to create CEL environment", "error", err)
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return &ConfigResolver{
		config: config,
		env:    env,
	}, nil
}

// Regular expression to match ${...} pattern anywhere in the string
var configRefPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// ResolveValue resolves a value that may contain one or more ${...} CEL expressions
// Supports:
//   - Single expression: "${config.timeout}" -> evaluates to the value
//   - Template strings: "timeout is ${config.timeout}s" -> "timeout is 30s"
//   - Multiple expressions: "${config.host}:${config.port}" -> "localhost:8080"
//
// If the value is not a string or contains no ${...} patterns, returns unchanged
// Returns an error if any config reference fails to resolve
func (r *ConfigResolver) ResolveValue(value interface{}) (interface{}, error) {
	// If no config loaded, return value as-is
	if r == nil || r.config == nil {
		return value, nil
	}

	// Only process string values
	strValue, ok := value.(string)
	if !ok {
		return value, nil
	}

	// Find all ${...} patterns in the string
	matches := configRefPattern.FindAllStringSubmatch(strValue, -1)
	if matches == nil {
		return value, nil // No config references found
	}

	// Check if the entire string is a single ${...} expression
	// If so, return the evaluated value directly (preserving type)
	if len(matches) == 1 && configRefPattern.MatchString(strValue) {
		fullMatch := configRefPattern.FindStringSubmatch(strValue)
		if fullMatch != nil && fullMatch[0] == strValue {
			celExpr := fullMatch[1]

			slog.Debug("Resolving single config reference",
				"reference", strValue,
				"celExpression", celExpr,
				"phase", "runtime")

			resolved, err := r.evaluateCEL(celExpr)
			if err != nil {
				slog.Error("Failed to resolve config reference",
					"reference", strValue,
					"celExpression", celExpr,
					"error", err,
					"phase", "runtime")
				return nil, fmt.Errorf("config not resolved: failed to evaluate %q: %w", strValue, err)
			}

			slog.Debug("Config reference resolved",
				"reference", strValue,
				"celExpression", celExpr,
				"resolvedValue", resolved,
				"phase", "runtime")

			return resolved, nil
		}
	}

	// Multiple expressions or template string - perform string substitution
	result := strValue
	replacements := make(map[string]string)

	slog.Debug("Resolving template with multiple config references",
		"template", strValue,
		"expressionCount", len(matches),
		"phase", "runtime")

	for _, match := range matches {
		placeholder := match[0] // Full match like "${config.timeout}"
		celExpr := match[1]     // Expression inside like "config.timeout"

		// Skip if already resolved
		if _, exists := replacements[placeholder]; exists {
			continue
		}

		// Evaluate the CEL expression
		resolved, err := r.evaluateCEL(celExpr)
		if err != nil {
			slog.Error("Failed to resolve config reference in template",
				"placeholder", placeholder,
				"celExpression", celExpr,
				"error", err,
				"phase", "runtime")
			return nil, fmt.Errorf("config not resolved: failed to evaluate %q in template %q: %w", placeholder, strValue, err)
		}

		// Convert resolved value to string for substitution
		resolvedStr := fmt.Sprintf("%v", resolved)
		replacements[placeholder] = resolvedStr

		slog.Debug("Config reference in template resolved",
			"placeholder", placeholder,
			"celExpression", celExpr,
			"resolvedValue", resolved,
			"phase", "runtime")
	}

	// Perform all substitutions
	for placeholder, resolvedStr := range replacements {
		result = regexp.MustCompile(regexp.QuoteMeta(placeholder)).ReplaceAllString(result, resolvedStr)
	}

	slog.Debug("Template resolution complete",
		"original", strValue,
		"resolved", result,
		"phase", "runtime")

	return result, nil
}

// evaluateCEL evaluates a CEL expression with the config as context
func (r *ConfigResolver) evaluateCEL(expression string) (interface{}, error) {
	if r.env == nil {
		return nil, fmt.Errorf("CEL environment not initialized")
	}

	// Parse the CEL expression
	ast, issues := r.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation error: %w", issues.Err())
	}

	// Create CEL program
	prg, err := r.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program creation error: %w", err)
	}

	// Evaluate the expression with the config
	// Note: CEL is case-sensitive, so keys must match exactly as they appear in the config
	out, _, err := prg.Eval(map[string]interface{}{
		"config": r.config,
	})
	if err != nil {
		return nil, fmt.Errorf("CEL evaluation error: %w", err)
	}

	// Convert CEL value back to Go value
	return celValueToGo(out), nil
}

// celValueToGo converts a CEL ref.Val to a Go value
func celValueToGo(val ref.Val) interface{} {
	if val == nil {
		return nil
	}

	// Check for error or unknown value
	if types.IsError(val) || types.IsUnknown(val) {
		if types.IsError(val) {
			slog.Warn("CEL value conversion encountered error value", "value", val)
		}
		return nil
	}

	// Convert to native Go value
	return val.Value()
}

// ResolveMap resolves all ${config} references in a map recursively
// Returns an error if any config reference fails to resolve
func (r *ConfigResolver) ResolveMap(m map[string]interface{}) (map[string]interface{}, error) {
	if r == nil || r.config == nil {
		return m, nil // No config, return as-is
	}

	result := make(map[string]interface{})
	for key, value := range m {
		resolved, err := r.resolveValueRecursive(value)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve config for key %q: %w", key, err)
		}
		result[key] = resolved
	}
	return result, nil
}

// resolveValueRecursive resolves ${config} references recursively in nested structures
// Returns an error if any config reference fails to resolve
func (r *ConfigResolver) resolveValueRecursive(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return r.ResolveValue(v)
	case map[string]interface{}:
		return r.ResolveMap(v)
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			resolved, err := r.resolveValueRecursive(item)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve config at array index %d: %w", i, err)
			}
			result[i] = resolved
		}
		return result, nil
	default:
		return value, nil
	}
}
