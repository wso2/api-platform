/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package mcpserver

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// apiSchemaOut is get_api_schema's answer, and what New builds for each API type.
type apiSchemaOut struct {
	APIType  string   `json:"apiType"`
	Required []string `json:"required"`
	Schema   object   `json:"schema"`
}

// get_api_schema. add has already refused an unknown apiType.
func (s *Server) getAPISchema(_ context.Context, _ caller, in apiTypeIn) (apiSchemaOut, error) {
	return s.schemas[in.APIType], nil
}

// loadAPISchemas builds each API type's payload schema from the Platform API's
// own OpenAPI file, so it cannot drift from the REST API.
func loadAPISchemas(path string) (map[string]apiSchemaOut, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mcp server: read the OpenAPI spec: %w", err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]any `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("mcp server: parse the OpenAPI spec: %w", err)
	}
	all := doc.Components.Schemas
	out := make(map[string]apiSchemaOut, len(apiTypes))
	for _, t := range apiTypes {
		if all[t.schema] == nil {
			return nil, fmt.Errorf("mcp server: schema %s for %s is not in the OpenAPI spec", t.schema, t.name)
		}
		sch, _ := inline(all, all[t.schema], map[string]bool{t.schema: true}).(map[string]any)
		out[t.name] = apiSchemaOut{APIType: t.name, Required: requiredFields(sch), Schema: sch}
	}
	return out, nil
}

const refPrefix = "#/components/schemas/"

// inline replaces every $ref in v with the schema it points to. A $ref back to
// a schema already being inlined is left as it is.
func inline(all map[string]any, v any, open map[string]bool) any {
	switch x := v.(type) {
	case map[string]any:
		if ref, ok := x["$ref"].(string); ok && strings.HasPrefix(ref, refPrefix) {
			name := strings.TrimPrefix(ref, refPrefix)
			if open[name] || all[name] == nil {
				return x
			}
			open[name] = true
			resolved := inline(all, all[name], open)
			delete(open, name)
			target, isMap := resolved.(map[string]any)
			if !isMap {
				return resolved
			}
			// OpenAPI 3.1 lets keys sit beside a $ref; they are the more specific, so they win.
			// The copy keeps the spec's own maps unwritten.
			out := maps.Clone(target)
			for k, val := range x {
				if k != "$ref" {
					out[k] = inline(all, val, open)
				}
			}
			return out
		}
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = inline(all, val, open)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = inline(all, val, open)
		}
		return out
	}
	return v
}

// requiredFields lists an inlined schema's required fields, through allOf.
func requiredFields(sch map[string]any) []string {
	out := []string{}
	required, _ := sch["required"].([]any)
	for _, r := range required {
		if name, ok := r.(string); ok && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	parts, _ := sch["allOf"].([]any)
	for _, part := range parts {
		if partSchema, ok := part.(map[string]any); ok {
			for _, name := range requiredFields(partSchema) {
				if !slices.Contains(out, name) {
					out = append(out, name)
				}
			}
		}
	}
	return out
}
