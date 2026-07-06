/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package dto

// OpenAPI 3.0 specification structs
type OpenAPI struct {
	OpenAPI    string                `json:"openapi"`
	Info       Info                  `json:"info"`
	Servers    []Server              `json:"servers,omitempty"`
	Paths      map[string]PathItem   `json:"paths"`
	Components *Components           `json:"components,omitempty"`
	Security   []SecurityRequirement `json:"security,omitempty"`
	Tags       []Tag                 `json:"tags,omitempty"`
}

type Info struct {
	Title       string   `json:"title"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Contact     *Contact `json:"contact,omitempty"`
	License     *License `json:"license,omitempty"`
}

type Contact struct {
	Name string `json:"name,omitempty"`
}

type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type Server struct {
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
}

type ServerVariable struct {
	Default     string   `json:"default"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

type PathItem struct {
	Get     *OpenAPIOperation `json:"get,omitempty"`
	Post    *OpenAPIOperation `json:"post,omitempty"`
	Put     *OpenAPIOperation `json:"put,omitempty"`
	Delete  *OpenAPIOperation `json:"delete,omitempty"`
	Options *OpenAPIOperation `json:"options,omitempty"`
	Head    *OpenAPIOperation `json:"head,omitempty"`
	Patch   *OpenAPIOperation `json:"patch,omitempty"`
	Trace   *OpenAPIOperation `json:"trace,omitempty"`
}

type OpenAPIOperation struct {
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	OperationID string                `json:"operationId,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []SecurityRequirement `json:"security,omitempty"`
}

type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"` // path, query, header, cookie
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Schema      Schema `json:"schema"`
}

type Schema struct {
	Type        string            `json:"type,omitempty"`
	Properties  map[string]Schema `json:"properties,omitempty"`
	Description string            `json:"description,omitempty"`
}

type RequestBody struct {
	Required bool                 `json:"required,omitempty"`
	Content  map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema Schema `json:"schema"`
}

type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

type Components struct {
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

type SecurityScheme struct {
	Type  string      `json:"type"`
	In    string      `json:"in,omitempty"`
	Name  string      `json:"name,omitempty"`
	Flows interface{} `json:"flows,omitempty"` // For OAuth2
}

type SecurityRequirement map[string][]string

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
