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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"gopkg.in/yaml.v3"
)

// restAPITypeValue is the type-agnostic path value for RestApi publications,
// the same constant handler.restAPITypeValue is defined from.
const restAPITypeValue = constants.PublicationAPITypeRestAPI

// publicationDefinitionField names the definition in a validation error's field list, so a
// client can tell a definition problem from one in the other content sent with a publish.
const publicationDefinitionField = "definition"

// definitionInvalid is a publish validation failure about the definition itself.
func definitionInvalid(message string) *apperror.Error {
	return apperror.APIPublicationValidationFailed.New(message).WithFieldErrors(
		[]apperror.FieldError{{Field: publicationDefinitionField, Message: message}})
}

// definitionValidator checks a definition body for one apiType. Receives the
// raw, untrimmed data exactly as validateDefinitionContent was called with;
// deciding what counts as empty (and whether that's acceptable) is each
// validator's own responsibility.
type definitionValidator func(contentType string, data []byte) error

// definitionValidators maps apiType to its definition validator. An apiType
// with no entry here cannot be published — add a new type's validator to this
// map to enable publishing it without changing any caller of
// validateDefinitionContent.
var definitionValidators = map[string]definitionValidator{
	restAPITypeValue:                       validateRestAPIDefinition,
	constants.PublicationAPITypeGraphQLAPI: validateGraphQLAPIDefinition,
	constants.PublicationAPITypeMCPProxy:   validateMCPProxyDefinition,
}

// validateDefinitionContent validates a definition before publish. An
// apiType with no registered validator is rejected; one with a registered
// validator requires a non-nil definition.
func validateDefinitionContent(apiType string, definition *model.PublicationContent) error {
	v, ok := definitionValidators[apiType]
	if !ok {
		return apperror.APIPublicationTypeUnsupported.New()
	}
	if definition == nil {
		return definitionInvalid("A definition is required to publish this API")
	}
	return v(definition.ContentType, definition.Content)
}

// validateRestAPIDefinition rejects a REST definition that is not a valid,
// non-empty OpenAPI 3.x document. Empty content ("" or "{}") is rejected too
// — LoadSpecDocument fails it cleanly on its own ("there is nothing in the
// spec, it's empty"), so no separate empty-check is needed here.
func validateRestAPIDefinition(contentType string, data []byte) error {
	if contentType != "application/json" && contentType != "application/yaml" {
		return definitionInvalid(
			"A REST API definition must be application/json or application/yaml")
	}
	sd, err := utils.LoadSpecDocument([]byte(strings.TrimSpace(string(data)))) // same function the Develop tab's validator uses
	if err != nil {
		return definitionInvalid(
			"The definition could not be parsed as an OpenAPI 3.x document.").WithLogMessage(err.Error())
	}
	result := utils.ValidateSpec(sd) // same function the Develop tab's validator uses
	if result.IsValid {
		return nil
	}
	msg := "The definition is not a valid OpenAPI 3.x document"
	if len(result.Errors) > 0 {
		msg += ": " + result.Errors[0].Message
		if len(result.Errors) > 1 {
			msg += fmt.Sprintf(" (and %d more)", len(result.Errors)-1)
		}
	}
	return definitionInvalid(msg)
}

// validateGraphQLAPIDefinition rejects a GraphQL definition that is not a
// valid SDL document defining a Query type — the same check the API's own
// create/update and the validate-schema endpoint apply (validateGraphQLSDL).
func validateGraphQLAPIDefinition(contentType string, data []byte) error {
	if contentType != "application/graphql" {
		return definitionInvalid(
			"A GraphQL API definition must be application/graphql")
	}
	issues := validateGraphQLSDL(string(data))
	if len(issues) == 0 {
		return nil
	}
	first := issues[0]
	msg := "The definition is not a valid GraphQL SDL document: " + first.Message
	if first.Line != nil && first.Column != nil {
		msg += fmt.Sprintf(" (line %d, column %d)", *first.Line, *first.Column)
	}
	return definitionInvalid(msg)
}

// mcpCapabilityKinds are the lists an MCP definition groups its capabilities under.
var mcpCapabilityKinds = []string{"tools", "resources", "prompts"}

// mcpFlatEntryKinds maps a flat definition entry's type to the list it belongs
// under, the same grouping the API Portal applies when it reads one.
var mcpFlatEntryKinds = map[string]string{"TOOL": "tools", "RESOURCE": "resources", "PROMPT": "prompts"}

const mcpDefinitionShapeMessage = "The definition must be an object with tools, resources and prompts lists, or a list of typed entries"

// validateMCPProxyDefinition rejects an MCP definition that does not list at least
// one tool, resource or prompt, each a named object. Both shapes the API Portal
// reads are accepted: an object of lists, or a flat list of TOOL/RESOURCE/PROMPT
// entries. Other keys are left alone.
func validateMCPProxyDefinition(contentType string, data []byte) error {
	var parsed any
	switch contentType {
	case "application/json":
		if err := json.Unmarshal(data, &parsed); err != nil {
			return definitionInvalid(mcpDefinitionShapeMessage)
		}
	case "application/yaml":
		// The portal refuses a multi-document file, so a second document is refused here too.
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		if err := decoder.Decode(&parsed); err != nil {
			return definitionInvalid(mcpDefinitionShapeMessage)
		}
		var next any
		if err := decoder.Decode(&next); !errors.Is(err, io.EOF) {
			return definitionInvalid("The definition must be a single YAML document")
		}
	default:
		return definitionInvalid(
			"An MCP proxy definition must be application/json or application/yaml")
	}
	definition, err := groupMCPDefinition(parsed)
	if err != nil {
		return err
	}

	capabilities := 0
	for _, kind := range mcpCapabilityKinds {
		raw, present := definition[kind]
		if !present || raw == nil {
			continue
		}
		items, ok := raw.([]any)
		if !ok {
			return definitionInvalid(
				fmt.Sprintf("The definition's %s must be a list", kind))
		}
		for i, item := range items {
			entry, ok := item.(map[string]any)
			name, _ := entry["name"].(string)
			if !ok || strings.TrimSpace(name) == "" {
				return definitionInvalid(
					fmt.Sprintf("Entry %d of the definition's %s must be an object with a name", i+1, kind))
			}
		}
		capabilities += len(items)
	}
	if capabilities == 0 {
		return definitionInvalid(
			"The definition must list at least one tool, resource or prompt")
	}
	return nil
}

// groupMCPDefinition returns the definition as an object of lists, grouping a flat
// list's entries by their type.
func groupMCPDefinition(parsed any) (map[string]any, error) {
	switch definition := parsed.(type) {
	case map[string]any:
		return definition, nil
	case []any:
		grouped := map[string]any{}
		for i, item := range definition {
			entry, _ := item.(map[string]any)
			entryType, _ := entry["type"].(string)
			kind, ok := mcpFlatEntryKinds[entryType]
			if !ok {
				return nil, definitionInvalid(fmt.Sprintf(
					"Entry %d of the definition must have a type of TOOL, RESOURCE or PROMPT", i+1))
			}
			items, _ := grouped[kind].([]any)
			grouped[kind] = append(items, entry)
		}
		return grouped, nil
	default:
		return nil, definitionInvalid(mcpDefinitionShapeMessage)
	}
}
