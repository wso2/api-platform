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
	"io"
	"net/http"
	"strings"
)

const (
	// maxGraphQLSDLUploadBytes bounds an uploaded SDL file — mirrors the 5 MiB
	// ceiling the CLI's standalone-gateway sdlFile path already uses
	// (cli/src/cmd/gateway/apply.go's maxGraphQLSDLFileBytes), so the limit is
	// consistent regardless of which onboarding surface supplied the file.
	maxGraphQLSDLUploadBytes = 5 << 20

	// graphQLSDLFileFormField and graphQLMetadataFormField are the
	// multipart/form-data field names documented on GraphQLAPIMultipartRequest
	// (resources/openapi.yaml).
	graphQLSDLFileFormField  = "sdlFile"
	graphQLMetadataFormField = "metadata"
)

// ParseGraphQLAPIMultipartRequest extracts the JSON "metadata" field and the
// optional "sdlFile" file part from a multipart/form-data GraphQL API
// create/update request. metadataJSON is always returned non-empty on
// success; sdl is empty when no file part was submitted (the caller falls
// back to metadata's own sdl/sdlUrl/introspection path in that case).
//
// The file part is read entirely in memory through a size-limited reader —
// never written to a temp file — and bounded independently of the reported
// Content-Length, per file-access.md directives 3/5.
func ParseGraphQLAPIMultipartRequest(r *http.Request) (metadataJSON []byte, sdl string, err error) {
	if err := r.ParseMultipartForm(maxGraphQLSDLUploadBytes); err != nil {
		return nil, "", fmt.Errorf("failed to parse multipart form: %w", err)
	}

	metadata := r.FormValue(graphQLMetadataFormField)
	if strings.TrimSpace(metadata) == "" {
		return nil, "", fmt.Errorf("missing required '%s' field in multipart form", graphQLMetadataFormField)
	}

	f, fileHeader, ferr := r.FormFile(graphQLSDLFileFormField)
	if ferr != nil {
		// sdlFile is optional — a caller may submit metadata-only over
		// multipart (e.g. for a client that always uses one content type),
		// relying on metadata's own sdlUrl or upstream introspection.
		return []byte(metadata), "", nil
	}
	defer f.Close()

	if fileHeader.Size > maxGraphQLSDLUploadBytes {
		return nil, "", fmt.Errorf("'%s' file exceeds the maximum allowed size of %d bytes", graphQLSDLFileFormField, maxGraphQLSDLUploadBytes)
	}
	// Bound the read independently of the (client-reported, so untrusted)
	// Size header above.
	data, rerr := io.ReadAll(io.LimitReader(f, maxGraphQLSDLUploadBytes+1))
	if rerr != nil {
		return nil, "", fmt.Errorf("failed to read '%s' file: %w", graphQLSDLFileFormField, rerr)
	}
	if int64(len(data)) > maxGraphQLSDLUploadBytes {
		return nil, "", fmt.Errorf("'%s' file exceeds the maximum allowed size of %d bytes", graphQLSDLFileFormField, maxGraphQLSDLUploadBytes)
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, "", fmt.Errorf("'%s' file is empty", graphQLSDLFileFormField)
	}

	return []byte(metadata), string(data), nil
}

// IsMultipartFormRequest reports whether r's Content-Type indicates a
// multipart/form-data body (a bare prefix check is correct and sufficient
// here — Content-Type is a same-request header the client sets, not a
// separately-untrusted routing input like a URL path per GO-AUTH-004).
func IsMultipartFormRequest(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data")
}
