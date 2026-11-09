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

import "strings"

// projectRule says whether an API type's list and payload take a project.
type projectRule int

const (
	projectRequired projectRule = iota
	projectOptional
	projectNone
)

// apiType is one of the Platform API's API families. They share one route
// pattern, so the API tools serve all of them: apiType picks the collection.
type apiType struct {
	name       string // the apiType value, e.g. "rest-api"
	collection string // e.g. "/rest-apis"
	schema     string // the OpenAPI schema create_api's payload is decoded as
	project    projectRule
	// hiddenInOutput lists fields left out of tool results because they are very large.
	hiddenInOutput []string
	// definition is the API definition route; empty when unsupported.
	definition string
	// publishable types can be published to an API Portal.
	publishable bool
}

// apiTypes are the types the tools serve. Each type also needs a direct adapter
// (direct.go).
var apiTypes = []apiType{
	{
		name:        "rest-api",
		collection:  "/rest-apis",
		schema:      "CreateRESTAPIRequest",
		project:     projectRequired,
		definition:  "GET {c}/{id}/openapi",
		publishable: true,
	},
	{
		// The REST API takes a GraphQL API as multipart, for an SDL file upload.
		// The tools take the same fields as JSON, with the SDL inline, from a URL,
		// or by introspection; the direct adapter calls the service, so no form is built.
		name:        "graphql-api",
		collection:  "/graphql-apis",
		schema:      "CreateGraphQLAPIRequest",
		project:     projectRequired,
		definition:  "GET {c}/{id}/sdl",
		publishable: true,
	},
	{
		name:       "llm-provider",
		collection: "/llm-providers",
		schema:     "LLMProvider",
		project:    projectNone,
		// The template's OpenAPI document is stored on every provider and can be hundreds of KB.
		hiddenInOutput: []string{"openapi"},
	},
	{
		name:           "llm-proxy",
		collection:     "/llm-proxies",
		schema:         "LLMProxy",
		project:        projectRequired,
		hiddenInOutput: []string{"openapi"},
	},
	{
		// The REST API can publish an MCP proxy too, but publish_api builds the
		// listing from the API's definition, which an MCP proxy does not have.
		name:       "mcp-proxy",
		collection: "/mcp-proxies",
		schema:     "MCPProxy",
		project:    projectRequired,
	},
	{
		name:       "agent-proxy",
		collection: "/agent-proxies",
		schema:     "A2AAgentProxy",
		project:    projectOptional,
	},
}

func apiTypeNamed(name string) (*apiType, error) {
	for i := range apiTypes {
		if apiTypes[i].name == name {
			return &apiTypes[i], nil
		}
	}
	return nil, fail("apiType %q is not supported", name)
}

// expand fills a tool's route templates with this type: {c} is its collection,
// {t} its name, and {def} its definition route.
func (t *apiType) expand(routes []string) []string {
	out := make([]string, 0, len(routes))
	for _, r := range routes {
		if r == "{def}" {
			r = t.definition
		}
		r = strings.ReplaceAll(r, "{c}", t.collection)
		out = append(out, strings.ReplaceAll(r, "{t}", t.name))
	}
	return out
}

// typedInput is an API tool's input: it names the API type.
type typedInput interface{ typeName() string }

// apiTypeIn is embedded in every API tool's input.
type apiTypeIn struct {
	APIType string `json:"apiType" jsonschema:"The API type."`
}

func (a apiTypeIn) typeName() string { return a.APIType }

// apiType returns the input's API type; add has already refused an unknown one.
func (a apiTypeIn) apiType() *apiType {
	t, _ := apiTypeNamed(a.APIType)
	return t
}
