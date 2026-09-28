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

package utils

import (
	"fmt"

	"github.com/pb33f/libopenapi"
	openapivalidator "github.com/pb33f/libopenapi-validator"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/wso2/api-platform/platform-api/api"
)

// SpecDocument holds a libopenapi-parsed OpenAPI 3.x document with format metadata
// and a pre-built typed model.
type SpecDocument struct {
	doc    libopenapi.Document
	raw    []byte // original input bytes, used for re-encoding
	isJSON bool
	V3     *libopenapi.DocumentModel[v3high.Document]
	errs   []error // model-build / validation errors
}

// IsJSONBytes returns true if data's first non-whitespace byte is '{'.
func IsJSONBytes(data []byte) bool {
	for _, b := range data {
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			continue
		}
		return b == '{'
	}
	return false
}

// LoadSpecDocument parses the spec with libopenapi, builds the typed model,
// and returns an error if the spec is not OpenAPI 3.x.
// Swagger 2.x specs are rejected with a validation error.
// Build/validation errors are stored in sd.errs.
func LoadSpecDocument(data []byte) (*SpecDocument, error) {
	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		return nil, fmt.Errorf("spec is neither valid JSON nor YAML, or is missing 'openapi' key: %w", err)
	}

	info := doc.GetSpecInfo()
	// SpecType is "openapi" for v3, "swagger" for v2.
	if info == nil || info.SpecType != "openapi" {
		return nil, fmt.Errorf("only OpenAPI 3.x specifications are supported; Swagger 2.x is not allowed")
	}

	sd := &SpecDocument{
		doc:    doc,
		raw:    data,
		isJSON: IsJSONBytes(data),
	}

	m, buildErrs := doc.BuildV3Model()
	if buildErrs != nil {
		sd.errs = append(sd.errs, buildErrs)
	}
	sd.V3 = m

	return sd, nil
}

// ValidateSpec validates the OpenAPI 3.x spec against the full OpenAPI JSON
// Meta-Schema via libopenapi-validator, which catches structural violations
// (unknown path item keys, paths without a leading "/", missing required
// fields, invalid $ref targets, etc.). Returns an api.ValidateOpenAPIResponse
// with Info (title/version) populated when valid.
func ValidateSpec(sd *SpecDocument) api.ValidateOpenAPIResponse {
	// Surface any model-build errors first ($ref resolution failures, etc.).
	// When present, skip the JSON Schema validator — NewValidator would also fail.
	if len(sd.errs) > 0 {
		errs := make([]api.OpenAPIValidationError, 0, len(sd.errs))
		for _, e := range sd.errs {
			errs = append(errs, api.OpenAPIValidationError{Message: e.Error()})
		}
		return api.ValidateOpenAPIResponse{IsValid: false, Errors: errs}
	}

	// NewValidator internally calls BuildV3Model and — critically — sets the
	// document field that ValidateDocument requires. NewValidatorFromV3Model does
	// not set that field, causing the "Document is not set" error.
	v, vErrs := openapivalidator.NewValidator(sd.doc)
	if vErrs != nil {
		errs := make([]api.OpenAPIValidationError, 0, len(vErrs))
		for _, e := range vErrs {
			errs = append(errs, api.OpenAPIValidationError{Message: e.Error()})
		}
		return api.ValidateOpenAPIResponse{IsValid: false, Errors: errs}
	}

	valid, valErrs := v.ValidateDocument()
	if !valid {
		errs := make([]api.OpenAPIValidationError, 0, len(valErrs))
		for _, e := range valErrs {
			if len(e.SchemaValidationErrors) > 0 {
				for _, sve := range e.SchemaValidationErrors {
					var path *string
					if p := sve.FieldPath; p != "" {
						path = &p
					}
					errs = append(errs, api.OpenAPIValidationError{
						Message: sve.Reason,
						Path:    path,
					})
				}
			} else {
				var path *string
				if p := e.SpecPath; p != "" {
					path = &p
				}
				errs = append(errs, api.OpenAPIValidationError{
					Message: e.Message,
					Path:    path,
				})
			}
		}
		return api.ValidateOpenAPIResponse{IsValid: false, Errors: errs}
	}
	// Validate that at least one API path is defined.
	if sd.V3.Model.Paths != nil && sd.V3.Model.Paths.PathItems != nil && sd.V3.Model.Paths.PathItems.Len() == 0 {
		return api.ValidateOpenAPIResponse{
			IsValid: false,
			Errors: []api.OpenAPIValidationError{
				{
					Message: "API contract must contain at least one operation.",
				},
			},
		}
	}
	result := api.ValidateOpenAPIResponse{IsValid: true, Errors: []api.OpenAPIValidationError{}}
	if sd.V3 != nil && sd.V3.Model.Info != nil {
		title, version := sd.V3.Model.Info.Title, sd.V3.Model.Info.Version
		result.Info = &api.OpenAPISpecInfo{Title: &title, Version: &version}
	}
	return result
}
