/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package templateengine

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"text/template"

	"github.com/wso2/api-platform/common/redact"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
)

// RenderResult holds the rendered config and the tracker with resolved secret values.
type RenderResult struct {
	// Config is the config with spec fields resolved (same concrete type as input).
	Config any
	// Tracker contains the resolved secret values for redaction.
	Tracker *redact.SecretTracker
}

// RenderError wraps a template rendering failure (e.g. missing secret, malformed template).
// It signals that the configuration is invalid as submitted — callers should treat this as a bad request.
type RenderError struct {
	Cause error
}

func (e *RenderError) Error() string {
	return fmt.Sprintf("failed to render configuration: %v", e.Cause)
}

func (e *RenderError) Unwrap() error {
	return e.Cause
}

// RenderSpec renders template expressions in the spec of a StoredConfig.
// It renders cfg.Configuration in place, leaving cfg.SourceConfiguration untouched (as stored in DB).
// cfg.SensitiveValues is populated with tracked resolved secret values for redaction.
//
// Callers must ensure cfg.Configuration holds the value to render before calling. For the deployment
// flow, both Configuration and SourceConfiguration are equal at the call site. For LLM configs in the
// event listener, hydration runs first and sets Configuration to a RestAPI, which is then rendered here.
//
// Returns *RenderError if rendering fails (e.g. missing secret, malformed template).
func RenderSpec(cfg *models.StoredConfig, secretResolver funcs.SecretResolver, logger *slog.Logger) error {
	if cfg.Configuration == nil {
		return nil
	}

	renderResult, err := renderSpec(cfg.Configuration, secretResolver, logger)
	if err != nil {
		return &RenderError{Cause: fmt.Errorf("failed to render config %q (kind=%s): %w", cfg.Handle, cfg.Kind, err)}
	}

	cfg.Configuration = renderResult.Config
	cfg.SensitiveValues = renderResult.Tracker.Values()
	return nil
}

// renderSpec renders Go template expressions in the "spec" portion of a parsed
// artifact config. Only spec fields are processed — apiVersion, kind, and
// metadata are left untouched.
//
// The config must be a struct with JSON tags (e.g., api.RestAPI, api.LLMProviderConfiguration).
// It is marshalled to a generic map, string values within the "spec" key are
// individually rendered through text/template, and the full map is unmarshalled
// back into a new instance of the original concrete type.
func renderSpec(config any, secretResolver funcs.SecretResolver, logger *slog.Logger) (*RenderResult, error) {
	tracker := redact.NewSecretTracker()

	// Marshal the entire config to a generic map so we can isolate "spec".
	configBytes, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var configMap map[string]any
	if err := json.Unmarshal(configBytes, &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config to map: %w", err)
	}

	specVal, ok := configMap["spec"]
	if !ok {
		// No spec field — nothing to render, return as-is.
		return &RenderResult{Config: config, Tracker: tracker}, nil
	}

	// Build the FuncMap for template rendering.
	deps := &funcs.Deps{
		SecretResolver: secretResolver,
		Tracker:        tracker,
		Logger:         logger,
	}
	funcMap := funcs.BuildFuncMap(deps)

	// Walk the spec value recursively, rendering template expressions in string values.
	renderedSpec, err := renderValue(specVal, funcMap)
	if err != nil {
		return nil, fmt.Errorf("failed to render spec templates: %w", err)
	}

	// Replace spec in the map with the rendered version.
	configMap["spec"] = renderedSpec

	// Re-marshal the full map and unmarshal into a new instance of the original type.
	mergedBytes, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rendered config map: %w", err)
	}

	// Create a new zero-value of the same concrete type via reflect and unmarshal.
	newPtr := reflect.New(reflect.TypeOf(config)).Interface()
	if err := json.Unmarshal(mergedBytes, newPtr); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rendered config into %T: %w", config, err)
	}

	// Dereference the pointer to return the value type (matching the input convention).
	result := reflect.ValueOf(newPtr).Elem().Interface()

	return &RenderResult{Config: result, Tracker: tracker}, nil
}

// renderValue recursively walks a JSON-decoded value and renders Go template
// expressions found in string values. Non-string values are returned as-is.
//
// Why walk the structure instead of rendering the whole spec as bytes:
// rendering at the leaf string level lets json.Marshal escape special characters
// (", \, newlines) automatically when the map is re-serialized. A byte-level
// render would require every template function to JSON-escape its output,
// which breaks function chaining (e.g. {{ secret "X" | upper }}) because
// intermediate functions would double-escape already-escaped input.
func renderValue(val any, funcMap template.FuncMap) (any, error) {
	switch v := val.(type) {
	case string:
		if !strings.Contains(v, "{{") {
			return v, nil
		}
		rendered, err := render([]byte(v), funcMap)
		if err != nil {
			return nil, err
		}
		return string(rendered), nil

	case map[string]any:
		result := make(map[string]any, len(v))
		for key, child := range v {
			rendered, err := renderValue(child, funcMap)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", key, err)
			}
			result[key] = rendered
		}
		return result, nil

	case []any:
		result := make([]any, len(v))
		for i, child := range v {
			rendered, err := renderValue(child, funcMap)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			result[i] = rendered
		}
		return result, nil

	default:
		// Numbers, booleans, nil — return as-is.
		return val, nil
	}
}
