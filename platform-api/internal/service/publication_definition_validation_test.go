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

package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

const validRESTDefinitionSpec = `{"openapi":"3.0.0","info":{"title":"t","version":"1"},"paths":{"/":{"get":{"responses":{"200":{"description":"ok"}}}}}}`

// contentOf builds the *model.PublicationContent validateDefinitionContent
// takes, so each test case only has to name a content-type and a body.
func contentOf(contentType, data string) *model.PublicationContent {
	return &model.PublicationContent{ContentType: contentType, Content: []byte(data)}
}

// TestValidateDefinitionContent_RestAPIRejectsEmpty checks that a REST
// definition must be a non-empty, valid OpenAPI 3.x document — "" and "{}"
// are rejected the same as any other invalid content, in both accepted
// content-types.
func TestValidateDefinitionContent_RestAPIRejectsEmpty(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		data        string
	}{
		{"json, truly empty", "application/json", ""},
		{"json, whitespace only", "application/json", "   \n\t"},
		{"json, empty object", "application/json", "{}"},
		{"yaml, truly empty", "application/yaml", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDefinitionContent(restAPITypeValue, contentOf(tt.contentType, tt.data))
			if err == nil || !strings.Contains(err.Error(), "OpenAPI 3.x document") {
				t.Fatalf("got %v, want an OpenAPI-3.x-document rejection", err)
			}
		})
	}
}

// TestValidateDefinitionContent_RestAPIRequiresDefinition checks that a nil
// definition (never saved, as opposed to saved-but-empty) is rejected before
// any per-type validator runs.
func TestValidateDefinitionContent_RestAPIRequiresDefinition(t *testing.T) {
	err := validateDefinitionContent(restAPITypeValue, nil)
	if err == nil || !strings.Contains(err.Error(), "definition is required") {
		t.Fatalf("got %v, want a definition-required rejection", err)
	}
}

// TestValidateDefinitionContent_UnregisteredApiTypeIsRejected checks that an
// apiType with no registered validator cannot be published — whatever the
// definition looks like, including none at all.
func TestValidateDefinitionContent_UnregisteredApiTypeIsRejected(t *testing.T) {
	tests := []struct {
		name       string
		definition *model.PublicationContent
	}{
		{"nil definition", nil},
		{"empty body", contentOf("application/json", "")},
		{"non-empty body", contentOf("application/json", "{}")},
		{"content valid for a registered type", contentOf("application/json", validRESTDefinitionSpec)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDefinitionContent("websub-api", tt.definition)
			if !apperror.APIPublicationTypeUnsupported.Is(err) {
				t.Fatalf("got %v, want an APIPublicationTypeUnsupported rejection", err)
			}
		})
	}
}

// TestValidateDefinitionContent_NamesTheDefinitionField checks that every way a definition can
// fail publish validation reports "definition" as the field, so a client can tell it from a
// problem in the other content sent with a publish.
func TestValidateDefinitionContent_NamesTheDefinitionField(t *testing.T) {
	tests := []struct {
		name       string
		apiType    string
		definition *model.PublicationContent
	}{
		{"rest, none saved", restAPITypeValue, nil},
		{"rest, empty", restAPITypeValue, contentOf("application/json", "{}")},
		{"rest, unparseable", restAPITypeValue, contentOf("application/json", "{not json")},
		{"rest, not OpenAPI", restAPITypeValue, contentOf("application/json", `{"openapi":"3.0.0"}`)},
		{"rest, wrong content type", restAPITypeValue, contentOf("application/xml", validRESTDefinitionSpec)},
		{"graphql, wrong content type", constants.PublicationAPITypeGraphQLAPI, contentOf("application/json", "type Query { hello: String }")},
		{"graphql, malformed", constants.PublicationAPITypeGraphQLAPI, contentOf("application/graphql", "type Query { hello: }")},
		{"mcp, none saved", constants.PublicationAPITypeMCPProxy, nil},
		{"mcp, wrong content type", constants.PublicationAPITypeMCPProxy, contentOf("application/graphql", `{"tools":[{"name":"a"}]}`)},
		{"mcp, not an object", constants.PublicationAPITypeMCPProxy, contentOf("application/json", `"text"`)},
		{"mcp, flat entry without a type", constants.PublicationAPITypeMCPProxy, contentOf("application/json", `[{"name":"a"}]`)},
		{"mcp, two yaml documents", constants.PublicationAPITypeMCPProxy, contentOf("application/yaml", "tools:\n  - name: a\n---\nfoo: 1\n")},
		{"mcp, kind is not a list", constants.PublicationAPITypeMCPProxy, contentOf("application/json", `{"tools":{"name":"a"}}`)},
		{"mcp, entry without a name", constants.PublicationAPITypeMCPProxy, contentOf("application/json", `{"tools":[{"description":"d"}]}`)},
		{"mcp, nothing listed", constants.PublicationAPITypeMCPProxy, contentOf("application/json", `{}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDefinitionContent(tt.apiType, tt.definition)

			var appErr *apperror.Error
			if !errors.As(err, &appErr) || !apperror.APIPublicationValidationFailed.Is(err) {
				t.Fatalf("got %v, want a publication validation failure", err)
			}
			want := []apperror.FieldError{{Field: "definition", Message: appErr.Message}}
			if len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0] != want[0] {
				t.Fatalf("got field errors %v, want %v", appErr.FieldErrors, want)
			}
		})
	}
}

// TestValidateDefinitionContent_GraphQLAPI checks the graphql-api validator:
// only an application/graphql SDL defining a Query type can be published.
func TestValidateDefinitionContent_GraphQLAPI(t *testing.T) {
	tests := []struct {
		name       string
		definition *model.PublicationContent
		wantErr    string
	}{
		{"valid SDL", contentOf("application/graphql", "type Query { hello: String }"), ""},
		{"nil definition", nil, "definition is required"},
		{"wrong content type", contentOf("application/json", "type Query { hello: String }"), "must be application/graphql"},
		{"empty SDL", contentOf("application/graphql", "  "), "SDL must not be empty"},
		{"malformed SDL", contentOf("application/graphql", "type Query { hello: }"), "not a valid GraphQL SDL document"},
		{"no Query type", contentOf("application/graphql", "type Book { title: String }"), "must define a Query type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDefinitionContent(constants.PublicationAPITypeGraphQLAPI, tt.definition)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("got %v, want no error", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got %v, want an error containing %q", err, tt.wantErr)
			}
			if apperror.APIPublicationTypeUnsupported.Is(err) {
				t.Fatalf("got %v, want graphql-api to be a supported publication type", err)
			}
		})
	}
}

// TestValidateDefinitionContent_RestAPIDelegates checks that a non-empty
// rest-api definition is dispatched to validateRestAPIDefinition.
func TestValidateDefinitionContent_RestAPIDelegates(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		data        string
		wantErr     string
	}{
		{"valid spec", "application/json", validRESTDefinitionSpec, ""},
		{"invalid spec", "application/json", `{"openapi":"3.0.0"}`, "not a valid OpenAPI 3.x document"},
		{"unsupported content-type, non-empty body", "application/xml", validRESTDefinitionSpec, "must be application/json or application/yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDefinitionContent(restAPITypeValue, contentOf(tt.contentType, tt.data))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDefinitionContent_MCPProxy(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		data        string
		wantErr     string
	}{
		{"json tools only", "application/json", `{"tools":[{"name":"search"}]}`, ""},
		{"json all three kinds", "application/json", `{"tools":[{"name":"a"}],"resources":[{"name":"b","uri":"x://b"}],"prompts":[{"name":"c"}]}`, ""},
		{"yaml", "application/yaml", "prompts:\n  - name: summarize\n", ""},
		{"unrelated keys are left alone", "application/json", `{"tools":[{"name":"a"}],"serverInfo":{"name":"x"}}`, ""},
		{"empty object", "application/json", `{}`, "at least one"},
		{"empty lists", "application/json", `{"tools":[],"resources":[],"prompts":[]}`, "at least one"},
		{"flat list, as the API Portal stores it", "application/yaml", "- type: TOOL\n  name: echo\n- type: PROMPT\n  name: greet\n", ""},
		{"flat entry with an unknown type", "application/json", `[{"type":"TOOL","name":"a"},{"type":"tool","name":"b"}]`, "Entry 2 of the definition must have a type"},
		{"flat entry without a name", "application/json", `[{"type":"RESOURCE","uri":"x://a"}]`, "Entry 1 of the definition's resources"},
		{"two yaml documents", "application/yaml", "tools:\n  - name: a\n---\nfoo: 1\n", "single YAML document"},
		{"not an object or list", "application/json", `"text"`, "object with tools"},
		{"invalid json", "application/json", `{`, "object with tools"},
		{"empty yaml", "application/yaml", "", "object with tools"},
		{"kind is not a list", "application/json", `{"tools":{"name":"a"}}`, "tools must be a list"},
		{"entry without a name", "application/json", `{"tools":[{"description":"d"}]}`, "Entry 1 of the definition's tools"},
		{"blank name", "application/json", `{"prompts":[{"name":"ok"},{"name":"  "}]}`, "Entry 2 of the definition's prompts"},
		{"entry is not an object", "application/json", `{"resources":["a"]}`, "Entry 1 of the definition's resources"},
		{"unsupported content type", "application/graphql", `{"tools":[{"name":"a"}]}`, "application/json or application/yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDefinitionContent(constants.PublicationAPITypeMCPProxy, contentOf(tt.contentType, tt.data))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("want the definition accepted, got %v", err)
				}
				return
			}
			if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want a validation failure containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateDefinitionContent_MCPProxyRequiresDefinition(t *testing.T) {
	err := validateDefinitionContent(constants.PublicationAPITypeMCPProxy, nil)
	if !apperror.APIPublicationValidationFailed.Is(err) {
		t.Fatalf("want a validation failure for a missing definition, got %v", err)
	}
}
