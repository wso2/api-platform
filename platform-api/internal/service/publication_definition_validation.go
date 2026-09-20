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
	"fmt"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// restAPITypeValue is the type-agnostic path value for RestApi publications,
// the same constant handler.restAPITypeValue is defined from.
const restAPITypeValue = constants.PublicationAPITypeRestAPI

// definitionValidator checks a definition body for one apiType. Receives the
// raw, untrimmed data exactly as validateDefinitionContent was called with;
// deciding what counts as empty (and whether that's acceptable) is each
// validator's own responsibility.
type definitionValidator func(contentType string, data []byte) error

// definitionValidators maps apiType to its definition validator. An apiType
// with no entry here is not validated — add a new type's validator to this
// map to extend coverage without changing any caller of
// validateDefinitionContent.
var definitionValidators = map[string]definitionValidator{
	restAPITypeValue: validateRestAPIDefinition,
}

// validateDefinitionContent validates a definition before publish. An
// apiType with no registered validator is skipped (nil definition allowed);
// one with a registered validator requires a non-nil definition.
func validateDefinitionContent(apiType string, definition *model.PublicationContent) error {
	v, ok := definitionValidators[apiType]
	if !ok {
		return nil
	}
	if definition == nil {
		return apperror.APIPublicationValidationFailed.New("A definition is required to publish this API")
	}
	return v(definition.ContentType, definition.Content)
}

// validateRestAPIDefinition rejects a REST definition that is not a valid,
// non-empty OpenAPI 3.x document. Empty content ("" or "{}") is rejected too
// — LoadSpecDocument fails it cleanly on its own ("there is nothing in the
// spec, it's empty"), so no separate empty-check is needed here.
func validateRestAPIDefinition(contentType string, data []byte) error {
	if contentType != "application/json" && contentType != "application/yaml" {
		return apperror.APIPublicationValidationFailed.New(
			"A REST API definition must be application/json or application/yaml")
	}
	sd, err := LoadSpecDocument([]byte(strings.TrimSpace(string(data)))) // same function the Develop tab's validator uses
	if err != nil {
		return apperror.APIPublicationValidationFailed.New(
			"The definition could not be parsed as an OpenAPI 3.x document.").WithLogMessage(err.Error())
	}
	result := ValidateSpec(sd) // same function the Develop tab's validator uses
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
	return apperror.APIPublicationValidationFailed.New(msg)
}
