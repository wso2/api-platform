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
	"fmt"
	"path"
	"strings"
	"unicode"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// portalDocument is a selected document placed at <Folder>/<FileName> under the ZIP's docs/.
// Content is filled in by loadPortalContent.
type portalDocument struct {
	ID       string
	Handle   string
	Folder   string
	FileName string
	Content  []byte
}

// resolveDocUUIDs resolves each document handle to its doc_uuid, scoped to
// artifactUUID (a handle can legitimately repeat across two APIs in the same
// org), rejecting any handle absent for this artifact and any selection the
// portal cannot hold.
func (s *PublicationService) resolveDocUUIDs(artifactUUID string, handles []string, orgUUID string) ([]string, error) {
	docs, err := s.resolvePortalDocuments(artifactUUID, handles, orgUUID)
	if err != nil {
		return nil, err
	}
	var uuids []string
	for _, d := range docs {
		uuids = append(uuids, d.ID)
	}
	return uuids, nil
}

// resolvePortalDocuments looks up the selected documents and places them, in handle order.
func (s *PublicationService) resolvePortalDocuments(artifactUUID string, handles []string, orgUUID string) ([]portalDocument, error) {
	if len(handles) == 0 {
		return nil, nil
	}
	resolved, err := s.documentRepo.GetDocumentsByHandles(artifactUUID, handles, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve document handles: %w", err)
	}
	docs, err := docsForHandles(handles, resolved)
	if err != nil {
		return nil, err
	}
	return placeDocuments(docs)
}

// docsForHandles returns the documents in handle order, or a validation error naming the
// handles with none.
func docsForHandles(handles []string, resolved map[string]*model.Document) ([]*model.Document, error) {
	docs := make([]*model.Document, 0, len(handles))
	var unresolved []string
	for _, h := range handles {
		if doc, ok := resolved[h]; ok {
			docs = append(docs, doc)
		} else {
			unresolved = append(unresolved, h)
		}
	}
	if len(unresolved) > 0 {
		return nil, apperror.APIPublicationValidationFailed.New(
			"docIds not found in the organization's documents: " + strings.Join(unresolved, ", "))
	}
	return docs, nil
}

// placeDocuments maps each document to its portal path, rejecting an unsafe type or file
// name and two documents that would land on the same path.
func placeDocuments(docs []*model.Document) ([]portalDocument, error) {
	placed := make([]portalDocument, 0, len(docs))
	// Case-insensitive because the portal may be on a case-insensitive store.
	owners := make(map[string]string, len(docs))
	for _, doc := range docs {
		folder := strings.TrimPrefix(doc.Type, constants.DocumentTypePrefix)
		if !isSafePathElement(folder) {
			return nil, apperror.APIPublicationValidationFailed.New(
				fmt.Sprintf("document %q has a type that cannot be published", doc.Handle))
		}
		fileName := portalFileName(doc)
		if !isSafePathElement(fileName) {
			return nil, apperror.APIPublicationValidationFailed.New(
				fmt.Sprintf("document %q has a file name that cannot be published", doc.Handle))
		}

		key := strings.ToLower(folder + "/" + fileName)
		if other, taken := owners[key]; taken {
			return nil, apperror.APIPublicationValidationFailed.New(
				fmt.Sprintf("documents %q and %q have the same type and file name", other, doc.Handle))
		}
		owners[key] = doc.Handle

		placed = append(placed, portalDocument{ID: doc.ID, Handle: doc.Handle, Folder: folder, FileName: fileName})
	}
	return placed, nil
}

// portalFileName is the stored file name without directories, or "<handle>.md" when unset.
func portalFileName(doc *model.Document) string {
	name := path.Base(strings.ReplaceAll(doc.FileName, `\`, "/"))
	if name == "." || name == "/" || name == ".." {
		return doc.Handle + ".md"
	}
	return name
}

// isSafePathElement reports whether s can be one folder or file name inside the
// content ZIP: non-empty, no separators, not a relative-path marker, no control
// characters.
func isSafePathElement(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool {
		return r == '/' || r == '\\' || unicode.IsControl(r)
	})
}
