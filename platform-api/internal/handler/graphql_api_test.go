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

package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
)

func newGraphQLAPIMultipartHandlerRequest(t *testing.T, metadata, sdlFileContent string, includeFile bool) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if metadata != "" {
		if err := w.WriteField("metadata", metadata); err != nil {
			t.Fatalf("failed to write metadata field: %v", err)
		}
	}
	if includeFile {
		fw, err := w.CreateFormFile("sdlFile", "schema.graphql")
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

const graphQLHandlerTestSDL = "type Query { countries: [String] }"

func TestDecodeCreateGraphQLAPIRequest_JSON(t *testing.T) {
	body := `{"displayName":"Countries","context":"/countries","version":"v1.0","projectId":"default-project","sdl":"type Query { x: String }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql-apis", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var out api.CreateGraphQLAPIRequest
	if err := decodeCreateGraphQLAPIRequest(req, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.DisplayName != "Countries" || out.Sdl == nil || *out.Sdl != "type Query { x: String }" {
		t.Errorf("unexpected decode result: %+v", out)
	}
}

func TestDecodeCreateGraphQLAPIRequest_Multipart_FileWinsOverMetadataSDLUrl(t *testing.T) {
	metadata := `{"displayName":"Countries","context":"/countries","version":"v1.0","projectId":"default-project","sdlUrl":"https://example.com/schema.graphql"}`
	req := newGraphQLAPIMultipartHandlerRequest(t, metadata, graphQLHandlerTestSDL, true)

	var out api.CreateGraphQLAPIRequest
	if err := decodeCreateGraphQLAPIRequest(req, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Sdl == nil || *out.Sdl != graphQLHandlerTestSDL {
		t.Errorf("expected sdl to come from the uploaded file, got %v", out.Sdl)
	}
	if out.SdlUrl != nil {
		t.Errorf("expected sdlUrl to be cleared when a file part is uploaded, got %v", *out.SdlUrl)
	}
	if out.DisplayName != "Countries" {
		t.Errorf("expected other metadata fields to still be populated, got %+v", out)
	}
}

func TestDecodeCreateGraphQLAPIRequest_Multipart_NoFile_PreservesMetadataFields(t *testing.T) {
	metadata := `{"displayName":"Countries","context":"/countries","version":"v1.0","projectId":"default-project","sdlUrl":"https://example.com/schema.graphql"}`
	req := newGraphQLAPIMultipartHandlerRequest(t, metadata, "", false)

	var out api.CreateGraphQLAPIRequest
	if err := decodeCreateGraphQLAPIRequest(req, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.SdlUrl == nil || *out.SdlUrl != "https://example.com/schema.graphql" {
		t.Errorf("expected sdlUrl from metadata to survive when no file part is uploaded, got %v", out.SdlUrl)
	}
	if out.Sdl != nil {
		t.Errorf("expected sdl to remain unset, got %v", *out.Sdl)
	}
}

func TestDecodeCreateGraphQLAPIRequest_Multipart_MissingMetadata(t *testing.T) {
	req := newGraphQLAPIMultipartHandlerRequest(t, "", graphQLHandlerTestSDL, true)

	var out api.CreateGraphQLAPIRequest
	if err := decodeCreateGraphQLAPIRequest(req, &out); err == nil {
		t.Fatal("expected an error when the metadata field is missing")
	}
}

func TestDecodeUpdateGraphQLAPIRequest_JSON(t *testing.T) {
	body := `{"displayName":"Countries","context":"/countries","version":"v1.0","sdl":"type Query { x: String }"}`
	req := httptest.NewRequest(http.MethodPut, "/graphql-apis/countries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var out api.GraphQLAPI
	if err := decodeUpdateGraphQLAPIRequest(req, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Sdl == nil || *out.Sdl != "type Query { x: String }" {
		t.Errorf("unexpected decode result: %+v", out)
	}
}

func TestDecodeUpdateGraphQLAPIRequest_Multipart_FileWinsOverMetadataSDLUrl(t *testing.T) {
	metadata := `{"displayName":"Countries","context":"/countries","version":"v1.0","sdlUrl":"https://example.com/schema.graphql"}`
	req := newGraphQLAPIMultipartHandlerRequest(t, metadata, graphQLHandlerTestSDL, true)

	var out api.GraphQLAPI
	if err := decodeUpdateGraphQLAPIRequest(req, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Sdl == nil || *out.Sdl != graphQLHandlerTestSDL {
		t.Errorf("expected sdl to come from the uploaded file, got %v", out.Sdl)
	}
	if out.SdlUrl != nil {
		t.Errorf("expected sdlUrl to be cleared when a file part is uploaded, got %v", *out.SdlUrl)
	}
}

func TestDecodeUpdateGraphQLAPIRequest_Multipart_MissingMetadata(t *testing.T) {
	req := newGraphQLAPIMultipartHandlerRequest(t, "", graphQLHandlerTestSDL, true)

	var out api.GraphQLAPI
	if err := decodeUpdateGraphQLAPIRequest(req, &out); err == nil {
		t.Fatal("expected an error when the metadata field is missing")
	}
}
