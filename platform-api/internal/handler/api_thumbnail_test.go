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

// Unit tests for the /apis/{apiType}/{apiId}/thumbnail handlers. They reuse the
// in-memory fakes and request helpers from api_document_test.go.

package handler

import (
	"bytes"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// Minimal payloads that net/http.DetectContentType classifies by magic bytes.
var (
	thumbPNG  = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)
	thumbJPEG = append([]byte("\xff\xd8\xff\xe0"), bytes.Repeat([]byte{0}, 32)...)
)

func thumbSeed(env *docHEnv, content []byte, contentType, fileName string) *model.Document {
	return env.seed(&model.Document{
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail,
		FileName:    fileName,
		ContentType: contentType,
		Content:     content,
	})
}

func thumbPut(t *testing.T, env *docHEnv, name string, content []byte) int {
	t.Helper()
	rec := env.form(t, http.MethodPut, docHThumbPath, nil, docHFile{field: "file", name: name, content: content})
	return rec.Code
}

func TestNewAPIThumbnailHandler_MaxBodyBytes(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Server
		want int64
	}{
		{"nil config uses the default", nil, constants.DefaultThumbnailMaxBytes},
		{"zero value uses the default", &config.Server{}, constants.DefaultThumbnailMaxBytes},
		{"configured value wins", &config.Server{ThumbnailMaxFetchBytes: 2048}, 2048},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewAPIThumbnailHandler(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), tc.cfg)
			if h.maxBodyBytes != tc.want {
				t.Errorf("maxBodyBytes = %d, want %d", h.maxBodyBytes, tc.want)
			}
		})
	}
}

func TestThumbnailRoutes_UnsupportedMethodsAreRejected(t *testing.T) {
	env := newDocHEnv(t, nil)
	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		if rec := env.get(t, method, docHThumbPath); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestThumbnailHandlers_MissingOrganizationIsUnauthorized(t *testing.T) {
	env := newDocHEnv(t, nil)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			rec := env.do(t, method, docHThumbPath, "", "", nil, "")
			docHAssertError(t, rec, http.StatusUnauthorized, apperror.CodeCommonUnauthorized)
		})
	}
}

func TestThumbnailHandlers_UnresolvableArtifactIsNotFound(t *testing.T) {
	env := newDocHEnv(t, nil)
	apiBase := constants.APIBasePath + "/apis/"
	cases := []struct{ name, target, org string }{
		{"unknown kind", apiBase + "bogus-kind/" + docHAPI + "/thumbnail", docHOrg},
		{"unknown api", apiBase + docHKind + "/no-such-api/thumbnail", docHOrg},
		{"api in another organization", docHThumbPath, "org-2"},
	}
	for _, tc := range cases {
		for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
			t.Run(tc.name+" "+method, func(t *testing.T) {
				rec := env.do(t, method, tc.target, tc.org, "", nil, "")
				docHAssertError(t, rec, http.StatusNotFound, apperror.CodeCommonNotFound)
			})
		}
	}
}

// ---------------------------------------------------------------------------
// GET /thumbnail
// ---------------------------------------------------------------------------

func TestGetThumbnail_StreamsStoredImage(t *testing.T) {
	env := newDocHEnv(t, nil)
	thumbSeed(env, thumbPNG, "image/png", "logo.png")

	rec := env.get(t, http.MethodGet, docHThumbPath)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), thumbPNG) {
		t.Errorf("body does not match the stored bytes")
	}
	wantHeaders := map[string]string{
		"Content-Type":           "image/png",
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "no-store", // bytes change in place at a fixed URL
		"Content-Disposition":    `inline; filename="logo.png"`,
	}
	for name, want := range wantHeaders {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestGetThumbnail_HeaderFallbacksAndEscaping(t *testing.T) {
	t.Run("missing content type falls back to octet-stream", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		thumbSeed(env, thumbPNG, "", "logo.png")
		rec := env.get(t, http.MethodGet, docHThumbPath)
		if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
			t.Errorf("Content-Type = %q, want application/octet-stream", got)
		}
	})
	t.Run("missing file name omits Content-Disposition", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		thumbSeed(env, thumbPNG, "image/png", "")
		rec := env.get(t, http.MethodGet, docHThumbPath)
		if got := rec.Header().Get("Content-Disposition"); got != "" {
			t.Errorf("Content-Disposition = %q, want none", got)
		}
	})
	t.Run("quotes and backslashes in the file name are escaped", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		thumbSeed(env, thumbPNG, "image/png", `lo"go\1.png`)
		rec := env.get(t, http.MethodGet, docHThumbPath)
		want := `inline; filename="lo\"go\\1.png"`
		if got := rec.Header().Get("Content-Disposition"); got != want {
			t.Errorf("Content-Disposition = %q, want %q", got, want)
		}
	})
}

func TestGetThumbnail_NoContentWhenNoneIsSet(t *testing.T) {
	t.Run("never uploaded", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		if rec := env.get(t, http.MethodGet, docHThumbPath); rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rec.Code)
		}
	})
	t.Run("stored row has no bytes", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		thumbSeed(env, nil, "image/png", "logo.png")
		if rec := env.get(t, http.MethodGet, docHThumbPath); rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rec.Code)
		}
	})
}

// Only the thumbnail row is ever served here, never a user document that
// happens to be stored under the reserved handle.
func TestGetThumbnail_IgnoresRowsOfAnotherType(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.seed(&model.Document{
		Type: constants.DocumentTypeDefinition, Handle: constants.DocumentHandleThumbnail,
		Content: []byte("openapi: 3.0.0"), ContentType: "application/yaml",
	})
	if rec := env.get(t, http.MethodGet, docHThumbPath); rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204 (no thumbnail row exists)", rec.Code)
	}
}

func TestGetThumbnail_RepositoryFailureIsInternalError(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.docs.getErr = errors.New("db down")
	docHAssertError(t, env.get(t, http.MethodGet, docHThumbPath),
		http.StatusInternalServerError, apperror.CodeCommonInternalError)
}

// ---------------------------------------------------------------------------
// PUT /thumbnail
// ---------------------------------------------------------------------------

func TestUpsertThumbnail_StoresSniffedImage(t *testing.T) {
	cases := []struct {
		name, fileName string
		content        []byte
		wantType       string
	}{
		{"png", "logo.png", thumbPNG, "image/png"},
		{"jpeg", "photo.jpg", thumbJPEG, "image/jpeg"},
		// The declared extension is never trusted; the bytes decide.
		{"jpeg bytes under a .png name", "misleading.png", thumbJPEG, "image/jpeg"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newDocHEnv(t, nil)
			if code := thumbPut(t, env, tc.fileName, tc.content); code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204", code)
			}
			if len(env.docs.upserted) != 1 {
				t.Fatalf("upserted %d rows, want 1", len(env.docs.upserted))
			}
			got := env.docs.upserted[0]
			if got.Type != constants.DocumentTypeThumbnail || got.Handle != constants.DocumentHandleThumbnail {
				t.Errorf("stored as type=%q handle=%q, want the thumbnail singleton", got.Type, got.Handle)
			}
			if got.DisplayName != constants.DocumentDisplayNameThumbnail {
				t.Errorf("displayName = %q", got.DisplayName)
			}
			if got.ContentType != tc.wantType {
				t.Errorf("contentType = %q, want %q", got.ContentType, tc.wantType)
			}
			if got.FileName != tc.fileName {
				t.Errorf("fileName = %q, want %q", got.FileName, tc.fileName)
			}
			if !bytes.Equal(got.Content, tc.content) {
				t.Errorf("stored bytes differ from the upload")
			}
			if got.ArtifactUUID != docHArtifact || got.OrganizationUUID != docHOrg {
				t.Errorf("stored under artifact=%q org=%q", got.ArtifactUUID, got.OrganizationUUID)
			}
			if call := docHAuditLast(t, env.audit); call.action != "CREATE" || call.resourceType != "api_thumbnail" {
				t.Errorf("audit = %+v, want CREATE api_thumbnail", call)
			}
		})
	}
}

func TestUpsertThumbnail_ReplacesExistingAndIsServedBack(t *testing.T) {
	env := newDocHEnv(t, nil)
	thumbSeed(env, thumbPNG, "image/png", "old.png")

	if code := thumbPut(t, env, "new.jpg", thumbJPEG); code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", code)
	}
	if call := docHAuditLast(t, env.audit); call.action != "UPDATE" || call.resourceType != "api_thumbnail" {
		t.Errorf("audit = %+v, want UPDATE api_thumbnail", call)
	}

	rec := env.get(t, http.MethodGet, docHThumbPath)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), thumbJPEG) {
		t.Fatalf("GET after PUT: status = %d, body mismatch = %v", rec.Code, !bytes.Equal(rec.Body.Bytes(), thumbJPEG))
	}
	if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", got)
	}
}

func TestUpsertThumbnail_RejectsInvalidUploads(t *testing.T) {
	cases := []struct {
		name, fileName string
		content        []byte
	}{
		{"gif", "anim.gif", append([]byte("GIF89a"), make([]byte, 16)...)},
		{"plain text", "notes.png", []byte("not an image at all")},
		{"html masquerading as png", "xss.png", []byte("<html><script>alert(1)</script></html>")},
		{"svg", "icon.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)},
		{"empty file", "empty.png", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newDocHEnv(t, nil)
			rec := env.form(t, http.MethodPut, docHThumbPath, nil,
				docHFile{field: "file", name: tc.fileName, content: tc.content})
			docHAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
			if len(env.docs.upserted) != 0 {
				t.Errorf("an invalid upload was stored")
			}
		})
	}
}

func TestUpsertThumbnail_RequiresFileField(t *testing.T) {
	env := newDocHEnv(t, nil)
	rec := env.form(t, http.MethodPut, docHThumbPath, map[string]string{"note": "no file here"})
	docHAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)

	// A file under the wrong field name is the same as no file.
	rec = env.form(t, http.MethodPut, docHThumbPath, nil, docHFile{field: "image", name: "logo.png", content: thumbPNG})
	docHAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
}

func TestUpsertThumbnail_NotMultipartIsBadRequest(t *testing.T) {
	env := newDocHEnv(t, nil)
	rec := env.do(t, http.MethodPut, docHThumbPath, docHOrg, "", bytes.NewReader(thumbPNG), "image/png")
	docHAssertError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
}

func TestUpsertThumbnail_OversizedFileIsPayloadTooLarge(t *testing.T) {
	env := newDocHEnv(t, &config.Server{ThumbnailMaxFetchBytes: 16})
	// A valid PNG header, but larger than the configured ceiling.
	if code := thumbPut(t, env, "big.png", thumbPNG); code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", code)
	}
	if len(env.docs.upserted) != 0 {
		t.Errorf("an oversized upload was stored")
	}
}

func TestUpsertThumbnail_OversizedBodyIsPayloadTooLarge(t *testing.T) {
	env := newDocHEnv(t, &config.Server{ThumbnailMaxFetchBytes: 16})
	big := append(append([]byte{}, thumbPNG...), bytes.Repeat([]byte{0}, (1<<20)+1024)...)
	rec := env.form(t, http.MethodPut, docHThumbPath, nil, docHFile{field: "file", name: "huge.png", content: big})
	docHAssertError(t, rec, http.StatusRequestEntityTooLarge, apperror.CodeCommonPayloadTooLarge)
}

func TestUpsertThumbnail_RepositoryFailureIsInternalError(t *testing.T) {
	env := newDocHEnv(t, nil)
	env.docs.upsertErr = errors.New("disk full")
	rec := env.form(t, http.MethodPut, docHThumbPath, nil, docHFile{field: "file", name: "logo.png", content: thumbPNG})
	docHAssertError(t, rec, http.StatusInternalServerError, apperror.CodeCommonInternalError)
}

func TestThumbnailWrites_UnresolvableActorIsInternalError(t *testing.T) {
	env := newDocHEnvWithIdentity(t, nil, docHFailingIdentityRepo{})
	thumbSeed(env, thumbPNG, "image/png", "logo.png")

	body, ct := docHForm(t, nil, docHFile{field: "file", name: "logo.png", content: thumbPNG})
	rec := env.do(t, http.MethodPut, docHThumbPath, docHOrg, "alice", body, ct)
	docHAssertError(t, rec, http.StatusInternalServerError, apperror.CodeCommonInternalError)

	rec = env.do(t, http.MethodDelete, docHThumbPath, docHOrg, "alice", nil, "")
	docHAssertError(t, rec, http.StatusInternalServerError, apperror.CodeCommonInternalError)

	if len(env.docs.upserted)+len(env.docs.deletedHandles) != 0 {
		t.Errorf("a write reached the repository despite the identity failure")
	}
}

// ---------------------------------------------------------------------------
// DELETE /thumbnail
// ---------------------------------------------------------------------------

func TestDeleteThumbnail_Success(t *testing.T) {
	env := newDocHEnv(t, nil)
	thumbSeed(env, thumbPNG, "image/png", "logo.png")

	rec := env.get(t, http.MethodDelete, docHThumbPath)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if len(env.docs.deletedHandles) != 1 ||
		env.docs.deletedHandles[0] != constants.DocumentHandleThumbnail ||
		env.docs.deletedTypes[0] != constants.DocumentTypeThumbnail {
		t.Errorf("deleted handles=%v types=%v, want the thumbnail singleton", env.docs.deletedHandles, env.docs.deletedTypes)
	}
	if call := docHAuditLast(t, env.audit); call.action != "DELETE" || call.resourceType != "api_thumbnail" {
		t.Errorf("audit = %+v, want DELETE api_thumbnail", call)
	}
	// Once removed, GET falls back to 204 so the UI shows the initials avatar.
	if rec := env.get(t, http.MethodGet, docHThumbPath); rec.Code != http.StatusNoContent {
		t.Errorf("GET after DELETE status = %d, want 204", rec.Code)
	}
}

func TestDeleteThumbnail_Failures(t *testing.T) {
	t.Run("nothing to delete", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		env.docs.deleteErr = sql.ErrNoRows
		docHAssertError(t, env.get(t, http.MethodDelete, docHThumbPath),
			http.StatusNotFound, apperror.CodeCommonNotFound)
	})
	t.Run("repository failure", func(t *testing.T) {
		env := newDocHEnv(t, nil)
		env.docs.deleteErr = errors.New("db down")
		docHAssertError(t, env.get(t, http.MethodDelete, docHThumbPath),
			http.StatusInternalServerError, apperror.CodeCommonInternalError)
	})
}

// Guard against the sniffing allowlist drifting: only PNG and JPEG are accepted.
func TestAllowedThumbnailContentTypes(t *testing.T) {
	want := map[string]bool{"image/png": true, "image/jpeg": true}
	if len(allowedThumbnailContentTypes) != len(want) {
		t.Errorf("allowlist = %v, want exactly %v", allowedThumbnailContentTypes, want)
	}
	for ct := range want {
		if !allowedThumbnailContentTypes[ct] {
			t.Errorf("%s missing from the allowlist", ct)
		}
	}
	for _, ct := range []string{"image/gif", "image/svg+xml", "image/webp", "text/html; charset=utf-8", ""} {
		if allowedThumbnailContentTypes[ct] {
			t.Errorf("%q must not be accepted as a thumbnail type", ct)
		}
	}
}
