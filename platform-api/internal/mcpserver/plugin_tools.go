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
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wso2/api-platform/platform-api/pdk"
)

var pluginRisks = map[pdk.MCPRisk]*mcp.ToolAnnotations{
	pdk.MCPRead:        read,
	pdk.MCPWrite:       write,
	pdk.MCPWriteRepeat: writeRepeat,
	pdk.MCPDestructive: destructiveRepeat,
}

// addPluginTools registers the plugins' tools after the built-in ones. A tool
// with a built-in tool's name replaces it.
func (s *Server) addPluginTools(srv *mcp.Server, tools []pdk.MCPTool) error {
	for _, tool := range tools {
		if tool.Name == "" || tool.Run == nil || len(tool.Routes) == 0 || pluginRisks[tool.Risk] == nil {
			return fmt.Errorf("mcp server: plugin tool %q needs a name, a Run function, routes and a risk", tool.Name)
		}
		// go-sdk panics on a schema that isn't an object, which a pointer or non-struct gives.
		inputType := reflect.TypeOf(tool.Input)
		if inputType == nil || inputType.Kind() != reflect.Struct {
			return fmt.Errorf("mcp server: plugin tool %q: the input must be a struct value", tool.Name)
		}
		schema, err := jsonschema.ForType(inputType, nil)
		if err != nil {
			return fmt.Errorf("mcp server: plugin tool %q: %w", tool.Name, err)
		}
		if tool.Risk == pdk.MCPDestructive && schema.Properties["confirm"] != nil {
			return fmt.Errorf("mcp server: plugin tool %q: a destructive tool gets confirm from the server; leave it out of the input", tool.Name)
		}
		apiType := schema.Properties["apiType"]
		// Only a string apiType is the platform's API type; any other type keeps its own schema.
		s.inputSchema(schema, apiType != nil && (apiType.Type == "string" || slices.Contains(apiType.Types, "string")))
		for name, title := range tool.Titles {
			if p := schema.Properties[name]; p != nil {
				p.Title = title
			}
		}
		add(s, srv, &mcp.Tool{Name: tool.Name, Title: tool.Title, Description: tool.Description,
			Annotations: pluginRisks[tool.Risk], InputSchema: schema}, tool.Routes,
			func(ctx context.Context, c caller, arguments json.RawMessage) (any, error) {
				return tool.Run(ctx, pdk.MCPCall{Arguments: arguments, Org: c.org, User: c.user})
			})
	}
	return nil
}
