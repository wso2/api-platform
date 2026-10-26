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

package pdk

import (
	"context"
	"encoding/json"
)

// MCPToolProvider is an optional interface a Plugin may implement to add tools
// to the MCP server. A tool with a built-in tool's name replaces it.
type MCPToolProvider interface {
	MCPTools() []MCPTool
}

// MCPRisk tells the MCP client whether to ask the user first.
type MCPRisk int

// The zero value is not a risk: a tool that leaves it out is refused at startup.
const (
	MCPRead        MCPRisk = iota + 1 // runs without asking
	MCPWrite                          // asks first
	MCPWriteRepeat                    // asks first; calling twice gives the same result
	MCPDestructive                    // always asks; calling twice gives the same result. Takes confirm=true
)

// MCPTool is one tool a plugin adds.
type MCPTool struct {
	Name        string
	Title       string
	Description string
	Risk        MCPRisk
	// Routes are the REST routes the tool stands for, "METHOD /path" under
	// /api/v0.9. Their scopes are checked as for a built-in tool.
	Routes []string
	// Input is a zero value of the tool's input struct. Its type gives the schema.
	// A destructive tool must not have a confirm input: the server adds it.
	Input  any
	Titles map[string]string // titles for inputs the Platform API does not title
	// Run returns the tool's result, a struct or a map. A *ClientError's message
	// reaches the caller; any other error is logged and becomes a generic message.
	Run func(ctx context.Context, call MCPCall) (any, error)
}

// MCPCall is one tool call, after sign-in and the input checks.
type MCPCall struct {
	Arguments json.RawMessage // already checked against the input schema; has confirm for a destructive tool
	Org       string          // the Platform API's organization UUID
	User      string          // the caller's internal user ID
}
