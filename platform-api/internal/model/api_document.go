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

package model

const DocumentTypeDefinition = "DEFINITION"

// IsSingletonDocumentType reports whether at most one document of the given
// type may exist per artifact. DEFINITION is the only singleton type; all
// other types (e.g. "HOW_TO") allow multiple documents per artifact.
func IsSingletonDocumentType(docType string) bool {
	return docType == DocumentTypeDefinition
}

// Document represents a stored document attached to an artifact (e.g. an OpenAPI spec).
type Document struct {
	ID               string
	ArtifactUUID     string
	OrganizationUUID string
	Type             string
	Handle           string
	DisplayName      string
	FileName         string
	ContentType      string
	Content          []byte
	CreatedBy        string
	UpdatedBy        string
}
