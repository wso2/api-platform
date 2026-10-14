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
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

const (
	// maxGraphQLSDLUploadBytes bounds an uploaded SDL file at 5 MiB, matching
	// defaultOpenAPISpecMaxFetchBytes (openapi_spec_fetcher.go) so the ceiling
	// is the same whether the schema arrives as a file upload or via sdlUrl.
	maxGraphQLSDLUploadBytes = 5 << 20

	// maxGraphQLMultipartRequestBytes bounds the whole multipart request body
	// (sdlFile part plus the metadata JSON field plus multipart
	// boundary/header framing) — maxGraphQLSDLUploadBytes alone is only the
	// in-memory threshold ParseMultipartForm uses before spilling file parts
	// to a temp file, not a ceiling on the request body itself.
	maxGraphQLMultipartRequestBytes = maxGraphQLSDLUploadBytes + (1 << 20) // +1 MiB overhead

	// graphQLSDLFileFormField and graphQLMetadataFormField are the
	// multipart/form-data field names documented on GraphQLAPIMultipartRequest
	// (resources/openapi.yaml).
	graphQLSDLFileFormField  = "sdlFile"
	graphQLMetadataFormField = "metadata"
)

// allowedSDLFileExtensions is the accepted-file-type allowlist for an
// uploaded SDL document (file-access.md directive 6 — allowlist, never a
// denylist), matching the same three extensions the upload UI offers.
var allowedSDLFileExtensions = map[string]bool{
	".graphql": true,
	".gql":     true,
	".json":    true,
}

// validateSDLFileName rejects a client-supplied filename outright rather
// than silently sanitizing it, and checks its extension against the
// allowlist above. The filename is never used to build a path —
// ParseGraphQLAPIMultipartRequest discards it immediately after this check
// and only the file's content ever reaches the caller — and Go's own
// mime/multipart.Part.FileName() (what net/http.Request.FormFile reads)
// already runs it through filepath.Base() before this function ever sees
// it, so a forward-slash traversal attempt ("../../etc/passwd.graphql")
// arrives here already reduced to a bare "passwd.graphql". The explicit
// checks below are a second, cheap layer on top of that: a `/`/`\` survives
// on some platforms' filepath.Base as a normal filename character (Go's
// stripping is OS-separator-aware, not universal), and rejecting outright
// on sight is more auditable than depending on that platform detail holding.
// The null-byte case is caught earlier still, by the multipart header
// parser itself, before FormFile ever returns.
func validateSDLFileName(rawFilename string) error {
	if strings.TrimSpace(rawFilename) == "" {
		return fmt.Errorf("'%s' filename is required", graphQLSDLFileFormField)
	}
	if strings.ContainsRune(rawFilename, 0) ||
		strings.ContainsAny(rawFilename, `/\`) ||
		strings.Contains(rawFilename, "..") {
		return fmt.Errorf("'%s' filename is not allowed", graphQLSDLFileFormField)
	}
	if ext := strings.ToLower(filepath.Ext(rawFilename)); !allowedSDLFileExtensions[ext] {
		return fmt.Errorf("'%s' file type is not supported; accepted types: .graphql, .gql, .json", graphQLSDLFileFormField)
	}
	return nil
}

// ParseGraphQLAPIMultipartRequest extracts the JSON "metadata" field and the
// optional "sdlFile" file part from a multipart/form-data GraphQL API
// create/update request. metadataJSON is always returned non-empty on
// success; sdl is empty when no file part was submitted (the caller falls
// back to metadata's own sdl/sdlUrl/introspection path in that case).
//
// The whole request body is bounded via http.MaxBytesReader independently of
// the reported Content-Length (file-access.md directive 5). A part smaller
// than maxGraphQLSDLUploadBytes is read into memory; ParseMultipartForm may
// still spill a larger part to a temp file (bounded by the same ceiling) —
// MultipartForm.RemoveAll cleans that up once parsing completes.
func ParseGraphQLAPIMultipartRequest(r *http.Request) (metadataJSON []byte, sdl string, err error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxGraphQLMultipartRequestBytes)
	if err := r.ParseMultipartForm(maxGraphQLSDLUploadBytes); err != nil {
		return nil, "", fmt.Errorf("failed to parse multipart form: %w", err)
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	metadata := r.FormValue(graphQLMetadataFormField)
	if strings.TrimSpace(metadata) == "" {
		return nil, "", fmt.Errorf("missing required '%s' field in multipart form", graphQLMetadataFormField)
	}

	f, fileHeader, ferr := r.FormFile(graphQLSDLFileFormField)
	if ferr != nil {
		if !errors.Is(ferr, http.ErrMissingFile) {
			return nil, "", fmt.Errorf("failed to read '%s' part: %w", graphQLSDLFileFormField, ferr)
		}
		// sdlFile is optional — a caller may submit metadata-only over
		// multipart (e.g. for a client that always uses one content type),
		// relying on metadata's own sdlUrl or upstream introspection.
		return []byte(metadata), "", nil
	}
	defer f.Close()

	if err := validateSDLFileName(fileHeader.Filename); err != nil {
		return nil, "", err
	}

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
