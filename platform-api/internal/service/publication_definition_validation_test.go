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
	"strings"
	"testing"

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

// TestValidateDefinitionContent_UnregisteredApiTypeSkipsValidation checks
// that an apiType with no registered validator is left unvalidated — empty,
// missing, or not — matching today's behavior for definitions this package
// has no rules for yet.
func TestValidateDefinitionContent_UnregisteredApiTypeSkipsValidation(t *testing.T) {
	if err := validateDefinitionContent("graphql-api", nil); err != nil {
		t.Fatalf("nil definition: want nil (no validator registered), got %v", err)
	}
	for _, data := range []string{"", "not a valid schema at all"} {
		if err := validateDefinitionContent("graphql-api", contentOf("application/graphql", data)); err != nil {
			t.Fatalf("data %q: want nil (no validator registered), got %v", data, err)
		}
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
