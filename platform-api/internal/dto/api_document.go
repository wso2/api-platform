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

package dto

// CreateAPIDocumentRequest carries the raw spec and metadata when persisting a new spec
// document for an API. The service fills in document type, handle, display name, and content type.
type CreateAPIDocumentRequest struct {
	Type             string
	Handle           string
	DisplayName      string
	FileName         string
	Content 		[]byte
}

// PutAPIDocumentRequest carries the raw spec and metadata when replacing an existing
// spec document for an API. The service fills in document type, handle, display name, and content type.
type PutAPIDocumentRequest struct {
	Type             string
	Handle           string
	DisplayName      string
	FileName         string
	Content 		[]byte
}

// APIDocumentContent is returned by GetDocument — the raw spec bytes ready to serve.
type APIDocumentContent struct {
	Content     []byte
	ContentType string
}
