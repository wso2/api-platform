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
func NewConfigResolver(config map[string]interface{}) *ConfigResolver {
	// Create CEL environment with config variable
	env, err := cel.NewEnv(
		cel.Variable("config", cel.DynType),
	)
	if err != nil {
		slog.Error("Failed to create CEL environment", "error", err)
		return &ConfigResolver{config: config}
	}

	return &ConfigResolver{
		config: config,
		env:    env,
	}
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
func (r *ConfigResolver) ResolveValue(value interface{}) interface{} {
	// If no config loaded, return value as-is
	if r == nil || r.config == nil {
		return value
	}

	// Only process string values
	strValue, ok := value.(string)
	if !ok {
		return value
	}

	// Find all ${...} patterns in the string
	matches := configRefPattern.FindAllStringSubmatch(strValue, -1)
	if matches == nil {
		return value // No config references found
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
				slog.Warn("Failed to resolve config reference, keeping as-is",
					"reference", strValue,
					"celExpression", celExpr,
					"error", err,
					"phase", "runtime")
				return value
			}

			slog.Debug("Config reference resolved",
				"reference", strValue,
				"celExpression", celExpr,
				"resolvedValue", resolved,
				"phase", "runtime")

			return resolved
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
			slog.Warn("Failed to resolve config reference in template, keeping as-is",
				"placeholder", placeholder,
				"celExpression", celExpr,
				"error", err,
				"phase", "runtime")
			continue // Keep the placeholder in the string
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

	return result
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
		return nil
	}

	// Convert to native Go value
	return val.Value()
}

// ResolveMap resolves all $config(...) references in a map recursively
func (r *ConfigResolver) ResolveMap(m map[string]interface{}) map[string]interface{} {
	if r == nil || r.config == nil {
		return m // No config, return as-is
	}

	result := make(map[string]interface{})
	for key, value := range m {
		result[key] = r.resolveValueRecursive(value)
	}
	return result
}

// resolveValueRecursive resolves $config references recursively in nested structures
func (r *ConfigResolver) resolveValueRecursive(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return r.ResolveValue(v)
	case map[string]interface{}:
		return r.ResolveMap(v)
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = r.resolveValueRecursive(item)
		}
		return result
	default:
		return value
	}
}
