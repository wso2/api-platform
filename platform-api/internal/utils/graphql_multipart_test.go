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
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newGraphQLMultipartRequest(t *testing.T, metadata, sdlFileContent string, includeFile bool) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if metadata != "" {
		if err := w.WriteField(graphQLMetadataFormField, metadata); err != nil {
			t.Fatalf("failed to write metadata field: %v", err)
		}
	}
	if includeFile {
		fw, err := w.CreateFormFile(graphQLSDLFileFormField, "schema.graphql")
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}
		if _, err := fw.Write([]byte(sdlFileContent)); err != nil {
			t.Fatalf("failed to write form file content: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/graphql-apis", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestParseGraphQLAPIMultipartRequest_MetadataAndFile(t *testing.T) {
	metadata := `{"displayName":"Countries","context":"/countries","version":"v1.0","projectId":"default-project"}`
	sdl := "type Query { countries: [String] }"
	req := newGraphQLMultipartRequest(t, metadata, sdl, true)

	gotMetadata, gotSDL, err := ParseGraphQLAPIMultipartRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(gotMetadata) != metadata {
		t.Errorf("metadata = %q, want %q", gotMetadata, metadata)
	}
	if gotSDL != sdl {
		t.Errorf("sdl = %q, want %q", gotSDL, sdl)
	}
}

func TestParseGraphQLAPIMultipartRequest_MetadataOnly(t *testing.T) {
	metadata := `{"displayName":"Countries","context":"/countries","version":"v1.0","projectId":"default-project"}`
	req := newGraphQLMultipartRequest(t, metadata, "", false)

	gotMetadata, gotSDL, err := ParseGraphQLAPIMultipartRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(gotMetadata) != metadata {
		t.Errorf("metadata = %q, want %q", gotMetadata, metadata)
	}
	if gotSDL != "" {
		t.Errorf("sdl = %q, want empty (no file part submitted)", gotSDL)
	}
}

func TestParseGraphQLAPIMultipartRequest_MissingMetadata(t *testing.T) {
	req := newGraphQLMultipartRequest(t, "", "type Query { x: String }", true)

	_, _, err := ParseGraphQLAPIMultipartRequest(req)
	if err == nil {
		t.Fatal("expected an error when the metadata field is missing")
	}
}

func TestParseGraphQLAPIMultipartRequest_EmptyFile(t *testing.T) {
	metadata := `{"displayName":"Countries"}`
	req := newGraphQLMultipartRequest(t, metadata, "   \n\t", true)

	_, _, err := ParseGraphQLAPIMultipartRequest(req)
	if err == nil {
		t.Fatal("expected an error for an empty (whitespace-only) sdlFile")
	}
}

func TestParseGraphQLAPIMultipartRequest_OversizedFile(t *testing.T) {
	metadata := `{"displayName":"Countries"}`
	oversized := strings.Repeat("a", maxGraphQLSDLUploadBytes+1)
	req := newGraphQLMultipartRequest(t, metadata, oversized, true)

	_, _, err := ParseGraphQLAPIMultipartRequest(req)
	if err == nil {
		t.Fatal("expected an error for an sdlFile exceeding the size ceiling")
	}
	if !strings.Contains(err.Error(), "exceeds the maximum allowed size") {
		t.Errorf("error = %q, want it to mention the size ceiling", err.Error())
	}
}

func TestIsMultipartFormRequest(t *testing.T) {
	jsonReq := httptest.NewRequest(http.MethodPost, "/graphql-apis", nil)
	jsonReq.Header.Set("Content-Type", "application/json")
	if IsMultipartFormRequest(jsonReq) {
		t.Error("expected application/json request to not be detected as multipart")
	}

	multipartReq := newGraphQLMultipartRequest(t, `{"a":1}`, "", false)
	if !IsMultipartFormRequest(multipartReq) {
		t.Error("expected multipart/form-data request to be detected as multipart")
	}
}
