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

import "mime/multipart"

// ValidateOpenAPIRequest represents the multipart form request for OpenAPI validation from UI
// This handles file uploads from web UI where users can either:
// 1. Upload an OpenAPI definition file (JSON/YAML)
// 2. Provide a URL to fetch the OpenAPI definition
// 3. Provide both (service will prioritize file upload over URL)
type ValidateOpenAPIRequest struct {
	URL        string                `form:"url"`        // Optional: URL to fetch OpenAPI definition
	Definition *multipart.FileHeader `form:"definition"` // Optional: Uploaded OpenAPI file (JSON/YAML)
}

// OpenAPIValidationResponse represents the response for OpenAPI validation
type OpenAPIValidationResponse struct {
	IsAPIDefinitionValid bool     `json:"isAPIDefinitionValid"`
	Errors               []string `json:"errors,omitempty"`
	API                  *API     `json:"api,omitempty"`
}

// ImportOpenAPIRequest represents the request for importing an OpenAPI definition
type ImportOpenAPIRequest struct {
	URL        string                `form:"url"`        // Optional: URL to fetch OpenAPI definition
	Definition *multipart.FileHeader `form:"definition"` // Optional: Uploaded OpenAPI file (JSON/YAML)
	API        API                   `form:"api"`        // API details for the imported definition
}
