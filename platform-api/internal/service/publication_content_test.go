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
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// contentDocumentRepo serves documents by handle.
type contentDocumentRepo struct {
	repository.DocumentRepository
	docs map[string]*model.Document
	// loads counts GetDocument calls when set.
	loads *int
	// resolveErr fails the handle lookup; loadErr fails the content load.
	resolveErr, loadErr error
	// removedOnLoad makes the content load find nothing, as if the document was deleted after
	// it was selected.
	removedOnLoad bool
}

func (r contentDocumentRepo) GetDocumentsByHandles(_ string, handles []string, _ string) (map[string]*model.Document, error) {
	if r.resolveErr != nil {
		return nil, r.resolveErr
	}
	found := map[string]*model.Document{}
	for _, h := range handles {
		if d, ok := r.docs[h]; ok {
			found[h] = &model.Document{ID: d.ID, Handle: d.Handle, Type: d.Type, DisplayName: d.DisplayName, FileName: d.FileName}
		}
	}
	return found, nil
}

func (r contentDocumentRepo) GetDocument(_, handle, _, _ string) (*model.Document, error) {
	if r.loads != nil {
		*r.loads++
	}
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	if r.removedOnLoad {
		return nil, nil
	}
	return r.docs[handle], nil
}

func newContentService(docs map[string]*model.Document) *PublicationService {
	return NewPublicationService(nil, nil, contentDocumentRepo{docs: docs}, nil, nil, nil, time.Minute, 0, nil)
}

func TestLoadPortalContent(t *testing.T) {
	docs := map[string]*model.Document{
		"guide": {ID: "uuid-guide", Handle: "guide", Type: "DOC_HowTo", FileName: "guide.md", Content: []byte("# guide")},
		"faq":   {ID: "uuid-faq", Handle: "faq", Type: "DOC_FAQ", FileName: "faq.md", Content: []byte("# faq")},
	}

	t.Run("packs the selected documents", func(t *testing.T) {
		content, err := newContentService(docs).loadPortalContent(portalContentSource{ArtifactUUID: "artifact", OrgUUID: "org", DocHandles: []string{"guide", "faq"}})
		if err != nil || content == nil {
			t.Fatalf("got %v, %v", content, err)
		}
		files := readArchive(t, content.Archive)
		if len(files) != 2 || files["content/docs/HowTo/guide.md"] != "# guide" || files["content/docs/FAQ/faq.md"] != "# faq" {
			t.Fatalf("unexpected archive layout: %v", files)
		}
	})

	t.Run("no selection means no content", func(t *testing.T) {
		content, err := newContentService(docs).loadPortalContent(portalContentSource{ArtifactUUID: "artifact", OrgUUID: "org", DocHandles: nil})
		if err != nil || content != nil {
			t.Fatalf("got %v, %v", content, err)
		}
	})

	t.Run("rejects an unknown handle", func(t *testing.T) {
		_, err := newContentService(docs).loadPortalContent(portalContentSource{ArtifactUUID: "artifact", OrgUUID: "org", DocHandles: []string{"guide", "nope"}})
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "nope") {
			t.Fatalf("want a validation error naming the handle, got %v", err)
		}
	})

	t.Run("rejects a file over the portal's limit", func(t *testing.T) {
		big := map[string]*model.Document{
			"big": {ID: "uuid-big", Handle: "big", Type: "DOC_HowTo", FileName: "big.md", Content: make([]byte, portalContentMaxFileBytes+1)},
		}
		_, err := newContentService(big).loadPortalContent(portalContentSource{ArtifactUUID: "artifact", OrgUUID: "org", DocHandles: []string{"big"}})
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "too large") {
			t.Fatalf("want a too large error, got %v", err)
		}
	})

	t.Run("rejects documents over the portal's total", func(t *testing.T) {
		many := map[string]*model.Document{}
		var handles []string
		for _, h := range []string{"a", "b", "c", "d", "e", "f"} {
			many[h] = &model.Document{ID: "uuid-" + h, Handle: h, Type: "DOC_HowTo", FileName: h + ".md", Content: make([]byte, portalContentMaxFileBytes)}
			handles = append(handles, h)
		}
		_, err := newContentService(many).loadPortalContent(portalContentSource{ArtifactUUID: "artifact", OrgUUID: "org", DocHandles: handles})
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "too large") {
			t.Fatalf("want a too large error, got %v", err)
		}
	})

	t.Run("rejects a document removed after it was selected", func(t *testing.T) {
		svc := NewPublicationService(nil, nil, contentDocumentRepo{docs: docs, removedOnLoad: true}, nil, nil, nil, time.Minute, 0, nil)
		_, err := svc.loadPortalContent(portalContentSource{ArtifactUUID: "artifact", OrgUUID: "org", DocHandles: []string{"guide"}})
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), `"guide" was removed`) {
			t.Fatalf("want a validation error naming the document, got %v", err)
		}
	})

	t.Run("reports a failure to load a document", func(t *testing.T) {
		loadErr := errors.New("db down")
		svc := NewPublicationService(nil, nil, contentDocumentRepo{docs: docs, loadErr: loadErr}, nil, nil, nil, time.Minute, 0, nil)
		_, err := svc.loadPortalContent(portalContentSource{ArtifactUUID: "artifact", OrgUUID: "org", DocHandles: []string{"guide"}})
		if !errors.Is(err, loadErr) || apperror.APIPublicationValidationFailed.Is(err) {
			t.Fatalf("want the repository failure, not a validation error, got %v", err)
		}
	})

	t.Run("rejects a selection of too many documents before loading any", func(t *testing.T) {
		many := map[string]*model.Document{}
		var handles []string
		for i := 0; i <= portalContentMaxEntries; i++ {
			h := fmt.Sprintf("doc-%d", i)
			many[h] = &model.Document{ID: "uuid-" + h, Handle: h, Type: "DOC_HowTo", FileName: h + ".md"}
			handles = append(handles, h)
		}
		var loads int
		svc := NewPublicationService(nil, nil, contentDocumentRepo{docs: many, loads: &loads}, nil, nil, nil, time.Minute, 0, nil)
		_, err := svc.loadPortalContent(portalContentSource{ArtifactUUID: "artifact", OrgUUID: "org", DocHandles: handles})
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "too large") {
			t.Fatalf("want a too large error, got %v", err)
		}
		if loads != 0 {
			t.Fatalf("loaded %d documents before refusing the selection", loads)
		}
	})
}

// callLog records the order of the portal calls of a publish.
type callLog struct {
	PortalPublisher
	calls      []string
	contentErr error
	archive    []byte
}

func (l *callLog) CreateMissingPlans(context.Context, *model.APIPortal, []*model.SubscriptionPlan) error {
	l.calls = append(l.calls, "plans")
	return nil
}

func (l *callLog) Publish(context.Context, *model.APIPortal, string, string, *model.Publication, *model.PublicationContent) error {
	l.calls = append(l.calls, "listing")
	return nil
}

func (l *callLog) PublishContent(_ context.Context, _ *model.APIPortal, _ string, content *PortalContent) error {
	l.calls = append(l.calls, "content")
	l.archive = content.Archive
	return l.contentErr
}

// publishFlowDocsPublicationRepo returns a draft that selects the "guide" document and
// records whether the draft was promoted.
type publishFlowDocsPublicationRepo struct {
	publishFlowPublicationRepo
	planUUIDs []string
	docUUIDs  []string
	thumbnail *model.PublicationContent
	promoted  *bool
	// thumbnailErr and landingPageErr fail the load of that content.
	thumbnailErr, landingPageErr error
}

func (r publishFlowDocsPublicationRepo) GetContent(draftUUID string, contentType model.PublicationContentType, orgUUID string) (*model.PublicationContent, error) {
	switch contentType {
	case model.PublicationContentTypeImage:
		return r.thumbnail, r.thumbnailErr
	case model.PublicationContentTypeMarketing:
		return nil, r.landingPageErr
	}
	return r.publishFlowPublicationRepo.GetContent(draftUUID, contentType, orgUUID)
}

func (r publishFlowDocsPublicationRepo) GetDraft(_, _, _ string) (*model.Publication, []model.PublicationSelection, []model.PublicationSelection, error) {
	return &model.Publication{UUID: "draft-uuid"}, testSelections(r.planUUIDs), testSelections(r.docUUIDs), nil
}

// testSelections turns "uuid-<handle>" pointers into selections.
func testSelections(uuids []string) []model.PublicationSelection {
	var selections []model.PublicationSelection
	for _, id := range uuids {
		handle := strings.TrimPrefix(id, "uuid-")
		selections = append(selections, model.PublicationSelection{UUID: id, Handle: handle, DisplayName: handle})
	}
	return selections
}

func (r publishFlowDocsPublicationRepo) PromoteDraftToPublication(_, _, _, _ string, _ time.Time, _ time.Time, _, _ []model.PublicationSelection) (*model.Publication, bool, error) {
	*r.promoted = true
	return &model.Publication{UUID: "draft-uuid"}, false, nil
}

func newPublishDocsService(pub PortalPublisher, docUUIDs []string, thumbnail *model.PublicationContent, promoted *bool) *PublicationService {
	docs := map[string]*model.Document{
		"guide": {ID: "uuid-guide", Handle: "guide", Type: "DOC_HowTo", FileName: "guide.md", Content: []byte("# guide")},
	}
	return NewPublicationService(
		publishFlowArtifactRepo{}, publishFlowPortalRepo{}, contentDocumentRepo{docs: docs},
		publicationTestPlans(), publishFlowDocsPublicationRepo{docUUIDs: docUUIDs, thumbnail: thumbnail, promoted: promoted},
		pub, time.Minute, 0, nil)
}

func TestPublish_PushesDocumentsAfterTheListing(t *testing.T) {
	var promoted bool
	pub := &callLog{}
	svc := newPublishDocsService(pub, []string{"uuid-guide"}, nil, &promoted)

	if _, _, err := svc.Publish(context.Background(), restAPITypeValue, "my-api", "my-portal", "org", "actor"); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if got := strings.Join(pub.calls, ","); got != "listing,content" {
		t.Fatalf("want the listing then its content, got %s", got)
	}
	if files := readArchive(t, pub.archive); files["content/docs/HowTo/guide.md"] != "# guide" {
		t.Fatalf("unexpected archive: %v", files)
	}
	if !promoted {
		t.Fatal("want the draft promoted")
	}
}

func TestPublish_PushesTheThumbnailWithoutDocuments(t *testing.T) {
	var promoted bool
	pub := &callLog{}
	thumbnail := &model.PublicationContent{ContentType: "image/png", FileName: "logo.png", Content: pngHeader}
	svc := newPublishDocsService(pub, nil, thumbnail, &promoted)

	if _, _, err := svc.Publish(context.Background(), restAPITypeValue, "my-api", "my-portal", "org", "actor"); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if got := strings.Join(pub.calls, ","); got != "listing,content" {
		t.Fatalf("want the listing then its content, got %s", got)
	}
	if files := readArchive(t, pub.archive); len(files) != 1 || files["content/web/api-icon.png"] != string(pngHeader) {
		t.Fatalf("unexpected archive: %v", files)
	}
}

func TestPublish_SkipsTheContentPushWithoutDocumentsOrThumbnail(t *testing.T) {
	var promoted bool
	pub := &callLog{}
	svc := newPublishDocsService(pub, nil, nil, &promoted)

	if _, _, err := svc.Publish(context.Background(), restAPITypeValue, "my-api", "my-portal", "org", "actor"); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if got := strings.Join(pub.calls, ","); got != "listing" {
		t.Fatalf("want only the listing push, got %s", got)
	}
}

func TestPublish_FailedContentPushDoesNotPromote(t *testing.T) {
	tests := []struct {
		name string
		err  error
		is   func(error) bool
	}{
		{"portal rejection", &PortalConflictError{Message: "rejected"}, apperror.APIPublicationPortalConflict.Is},
		{"portal unavailable", errors.New("connection refused"), apperror.APIPublicationPortalUnavailable.Is},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var promoted bool
			pub := &callLog{contentErr: tc.err}
			svc := newPublishDocsService(pub, []string{"uuid-guide"}, nil, &promoted)

			published, _, err := svc.Publish(context.Background(), restAPITypeValue, "my-api", "my-portal", "org", "actor")
			if !tc.is(err) || published != nil {
				t.Fatalf("want the mapped portal error and no publication, got %v, %v", published, err)
			}
			if promoted {
				t.Fatal("the draft must stay unpromoted so a retry can finish the push")
			}
		})
	}
}

func TestPublish_RejectsAnOversizeSelectionBeforeAnyPortalCall(t *testing.T) {
	var promoted bool
	pub := &callLog{}
	svc := newPublishDocsService(pub, []string{"uuid-guide"}, nil, &promoted)
	svc.archiveMaxBytes = 1

	_, _, err := svc.Publish(context.Background(), restAPITypeValue, "my-api", "my-portal", "org", "actor")
	if !apperror.APIPublicationValidationFailed.Is(err) {
		t.Fatalf("want a validation error, got %v", err)
	}
	if len(pub.calls) != 0 {
		t.Fatalf("want no portal call, got %v", pub.calls)
	}
}

func TestPublish_StopsBeforeAnyPortalCallWhenTheContentCannotBeLoaded(t *testing.T) {
	loadErr := errors.New("db down")
	inactiveGold := &publicationPlanRepo{plans: map[string]*model.SubscriptionPlan{
		"gold": {UUID: "uuid-gold", Handle: "gold", Status: model.SubscriptionPlanStatusInactive},
	}}
	guide := map[string]*model.Document{
		"guide": {ID: "uuid-guide", Handle: "guide", Type: "DOC_HowTo", FileName: "guide.md"},
	}
	tests := []struct {
		name  string
		repo  publishFlowDocsPublicationRepo
		plans *publicationPlanRepo
		docs  contentDocumentRepo
		is    func(error) bool
	}{
		{"inactive plan", publishFlowDocsPublicationRepo{planUUIDs: []string{"uuid-gold"}}, inactiveGold, contentDocumentRepo{},
			apperror.APIPublicationValidationFailed.Is},
		{"thumbnail failure", publishFlowDocsPublicationRepo{thumbnailErr: loadErr}, publicationTestPlans(), contentDocumentRepo{},
			func(err error) bool { return errors.Is(err, loadErr) }},
		{"landing page failure", publishFlowDocsPublicationRepo{landingPageErr: loadErr}, publicationTestPlans(), contentDocumentRepo{},
			func(err error) bool { return errors.Is(err, loadErr) }},
		{"document removed", publishFlowDocsPublicationRepo{docUUIDs: []string{"uuid-guide"}}, publicationTestPlans(), contentDocumentRepo{docs: guide, removedOnLoad: true},
			apperror.APIPublicationValidationFailed.Is},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var promoted bool
			tc.repo.promoted = &promoted
			pub := &callLog{}
			svc := NewPublicationService(
				publishFlowArtifactRepo{}, publishFlowPortalRepo{}, tc.docs,
				tc.plans, tc.repo, pub, time.Minute, 0, nil)

			published, _, err := svc.Publish(context.Background(), restAPITypeValue, "my-api", "my-portal", "org", "actor")
			if !tc.is(err) || published != nil {
				t.Fatalf("want the load error and no publication, got %v, %v", published, err)
			}
			if len(pub.calls) != 0 || promoted {
				t.Fatalf("want no portal call and no promotion, got calls %v, promoted %v", pub.calls, promoted)
			}
		})
	}
}
