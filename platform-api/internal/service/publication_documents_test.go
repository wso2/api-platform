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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func TestValidateAndResolveDocUUIDs(t *testing.T) {
	docs := map[string]*model.Document{
		"guide": {ID: "uuid-guide", Handle: "guide", Type: "DOC_HowTo", FileName: "guide.md"},
		"faq":   {ID: "uuid-faq", Handle: "faq", Type: "DOC_FAQ", FileName: "faq.md"},
	}
	newService := func(repo contentDocumentRepo) *PublicationService {
		return NewPublicationService(nil, nil, repo, nil, nil, nil, time.Minute, 0, nil)
	}

	t.Run("returns the UUIDs in handle order", func(t *testing.T) {
		uuids, err := newService(contentDocumentRepo{docs: docs}).validateAndResolveDocUUIDs("artifact", []string{"faq", "guide"}, "org")
		if err != nil || strings.Join(uuids, ",") != "uuid-faq,uuid-guide" {
			t.Fatalf("got %v, %v", uuids, err)
		}
	})

	t.Run("no selection resolves to nothing", func(t *testing.T) {
		uuids, err := newService(contentDocumentRepo{resolveErr: errors.New("must not be queried")}).validateAndResolveDocUUIDs("artifact", nil, "org")
		if err != nil || uuids != nil {
			t.Fatalf("got %v, %v", uuids, err)
		}
	})

	t.Run("rejects an unknown handle", func(t *testing.T) {
		_, err := newService(contentDocumentRepo{docs: docs}).validateAndResolveDocUUIDs("artifact", []string{"guide", "nope"}, "org")
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "nope") {
			t.Fatalf("want a validation error naming the handle, got %v", err)
		}
	})

	t.Run("reports a repository failure", func(t *testing.T) {
		lookupErr := errors.New("db down")
		_, err := newService(contentDocumentRepo{resolveErr: lookupErr}).validateAndResolveDocUUIDs("artifact", []string{"guide"}, "org")
		if !errors.Is(err, lookupErr) || apperror.APIPublicationValidationFailed.Is(err) {
			t.Fatalf("want the repository failure, not a validation error, got %v", err)
		}
	})
}

func TestSaveDraftDetails_RejectsAnUnknownDocument(t *testing.T) {
	svc := NewPublicationService(publishFlowArtifactRepo{}, publishFlowPortalRepo{}, contentDocumentRepo{}, nil, nil, nil, time.Minute, 0, nil)
	draft := &model.Publication{DisplayName: "My API", Version: "1.0"}

	_, err := svc.SaveDraftDetails(restAPITypeValue, "my-api", "my-portal", "org", "actor", draft, nil, []string{"nope"})
	if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("want a validation error naming the handle, got %v", err)
	}
}

func TestDocsForHandles(t *testing.T) {
	resolved := map[string]*model.Document{
		"guide": {ID: "uuid-guide", Handle: "guide"},
		"faq":   {ID: "uuid-faq", Handle: "faq"},
	}

	docs, err := docsForHandles([]string{"faq", "guide"}, resolved)
	if err != nil || len(docs) != 2 || docs[0].ID != "uuid-faq" || docs[1].ID != "uuid-guide" {
		t.Fatalf("want documents in handle order, got %v, %v", docs, err)
	}

	_, err = docsForHandles([]string{"guide", "nope", "gone"}, resolved)
	if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "nope, gone") {
		t.Fatalf("want a validation error listing the missing handles, got %v", err)
	}
}

func TestPlaceDocuments(t *testing.T) {
	doc := func(handle, docType, fileName string) *model.Document {
		return &model.Document{ID: "uuid-" + handle, Handle: handle, Type: docType, FileName: fileName}
	}

	t.Run("places each document under its type folder", func(t *testing.T) {
		placed, err := placeDocuments([]*model.Document{
			doc("guide", "DOC_HowTo", "guide.md"),
			doc("faq", "DOC_FAQ", "faq.md"),
		})
		if err != nil || len(placed) != 2 {
			t.Fatalf("got %v, %v", placed, err)
		}
		if placed[0].Folder != "HowTo" || placed[0].FileName != "guide.md" || placed[0].ID != "uuid-guide" {
			t.Errorf("first = %+v", placed[0])
		}
		if placed[1].Folder != "FAQ" || placed[1].FileName != "faq.md" {
			t.Errorf("second = %+v", placed[1])
		}
	})

	t.Run("keeps only the base of a file name", func(t *testing.T) {
		for _, name := range []string{"../../guide.md", "a/b/guide.md", `a\b\guide.md`} {
			placed, err := placeDocuments([]*model.Document{doc("guide", "DOC_HowTo", name)})
			if err != nil || placed[0].FileName != "guide.md" {
				t.Errorf("%q: got %+v, %v", name, placed, err)
			}
		}
	})

	t.Run("falls back to the handle when there is no file name", func(t *testing.T) {
		for _, name := range []string{"", "..", "/"} {
			placed, err := placeDocuments([]*model.Document{doc("guide", "DOC_HowTo", name)})
			if err != nil || placed[0].FileName != "guide.md" {
				t.Errorf("%q: got %+v, %v", name, placed, err)
			}
		}
	})

	t.Run("rejects a type that is not one folder name", func(t *testing.T) {
		for _, docType := range []string{"DOC_a/b", `DOC_a\b`, "DOC_..", "DOC_", "DOC_a\x00b"} {
			_, err := placeDocuments([]*model.Document{doc("guide", docType, "guide.md")})
			if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "type") {
				t.Errorf("%q: want a type validation error, got %v", docType, err)
			}
		}
	})

	t.Run("rejects a file name with a control character", func(t *testing.T) {
		_, err := placeDocuments([]*model.Document{doc("guide", "DOC_HowTo", "gu\nide.md")})
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "file name") {
			t.Fatalf("want a file name validation error, got %v", err)
		}
	})

	t.Run("rejects a name with two dots in a row, which the portal refuses", func(t *testing.T) {
		_, err := placeDocuments([]*model.Document{doc("guide", "DOC_HowTo", "release..notes.md")})
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "file name") {
			t.Fatalf("want a file name validation error, got %v", err)
		}
		_, err = placeDocuments([]*model.Document{doc("guide", "DOC_Release..Notes", "guide.md")})
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "type") {
			t.Fatalf("want a type validation error, got %v", err)
		}
	})

	t.Run("rejects two documents on the same path ignoring case", func(t *testing.T) {
		_, err := placeDocuments([]*model.Document{
			doc("guide", "DOC_HowTo", "Guide.md"),
			doc("guide-2", "DOC_HOWTO", "guide.MD"),
		})
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), `"guide" and "guide-2"`) {
			t.Fatalf("want a collision error naming both handles, got %v", err)
		}
	})

	t.Run("the same file name under different types is fine", func(t *testing.T) {
		if _, err := placeDocuments([]*model.Document{
			doc("guide", "DOC_HowTo", "readme.md"),
			doc("samples", "DOC_Samples", "readme.md"),
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
