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

package repository

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

// seedTestArtifact creates org + project + artifact rows so a document row
// can satisfy the api_documents.artifact_uuid foreign key. Returns the
// artifact UUID the test should attach documents to.
func seedTestArtifact(t *testing.T, db *database.DB, orgUUID, artifactUUID string) {
	t.Helper()

	if _, err := db.Exec(
		`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		 VALUES (?, ?, ?, 'default', 'idp-ref', datetime('now'), datetime('now'))`,
		orgUUID, "test-org-"+orgUUID, "Test Org",
	); err != nil {
		t.Fatalf("seed organization: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO projects (uuid, handle, display_name, organization_uuid, created_at, updated_at)
		 VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		"project-"+artifactUUID, "test-project-"+artifactUUID, "Test Project", orgUUID,
	); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO artifacts (uuid, type, organization_uuid)
		 VALUES (?, ?, ?)`,
		artifactUUID, constants.RestApi, orgUUID,
	); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
}

// seedExtraArtifact adds a project + artifact row under an organization that
// was already seeded by a prior seedTestArtifact call. Use when a test needs
// a second artifact in the SAME org — a second seedTestArtifact call would
// try to re-insert the org row and fail the handle unique index.
func seedExtraArtifact(t *testing.T, db *database.DB, orgUUID, artifactUUID string) {
	t.Helper()

	if _, err := db.Exec(
		`INSERT INTO projects (uuid, handle, display_name, organization_uuid, created_at, updated_at)
		 VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		"project-"+artifactUUID, "test-project-"+artifactUUID, "Test Project", orgUUID,
	); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO artifacts (uuid, type, organization_uuid)
		 VALUES (?, ?, ?)`,
		artifactUUID, constants.RestApi, orgUUID,
	); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
}

// insertDocumentRow writes a document row directly without going through the
// repo — lets a test seed a THUMBNAIL/DEFINITION row to assert the repo's
// reserved-type guards without first testing the write path those guards cover.
func insertDocumentRow(t *testing.T, db *database.DB, doc *model.Document) {
	t.Helper()
	// api_documents.content is NOT NULL — default to an empty-but-non-nil blob
	// so metadata-only seeds don't have to supply filler bytes.
	content := doc.Content
	if content == nil {
		content = []byte{}
	}
	if _, err := db.Exec(
		`INSERT INTO api_documents (uuid, artifact_uuid, organization_uuid, type, handle, display_name, file_name, content_type, content, created_by, created_at, updated_by, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), ?, datetime('now'))`,
		doc.ID, doc.ArtifactUUID, doc.OrganizationUUID, doc.Type, doc.Handle, doc.DisplayName,
		doc.FileName, doc.ContentType, content, doc.CreatedBy, doc.UpdatedBy,
	); err != nil {
		t.Fatalf("insert document row: %v", err)
	}
}

// ListDocumentsByArtifact EXCLUDE reserved types (THUMBNAIL, DEFINITION)
func TestDocumentRepo_ListDocumentsByArtifact_ExcludesReservedTypes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-list-reserved"
	const artifactUUID = "artifact-list-reserved"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	// Seed one of each reserved type + one legitimate user doc.
	insertDocumentRow(t, db, &model.Document{
		ID: "doc_thumbnail", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypeThumbnail, Handle: constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail, Content: []byte{0x89, 'P'},
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "doc_definition", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypeDefinition, Handle: constants.DocumentHandleDefinition,
		DisplayName: constants.DocumentDisplayNameDefinition, Content: []byte("{}"),
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "user-howto", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypeHowTo, Handle: "overview",
		DisplayName: "Overview", Content: []byte("# Overview"),
	})

	repo := NewDocumentRepo(db)

	docs, total, err := repo.ListDocumentsByArtifact(artifactUUID, orgUUID, "", 10, 0)
	if err != nil {
		t.Fatalf("ListDocumentsByArtifact: %v", err)
	}
	if total != 1 || len(docs) != 1 {
		t.Fatalf("expected only the HOW_TO row (1 total), got total=%d docs=%d", total, len(docs))
	}
	if docs[0].Handle != "overview" {
		t.Errorf("expected the HOW_TO row to be returned, got handle=%q (type=%q)", docs[0].Handle, docs[0].Type)
	}
}

// CreateDocument enforces UNIQUE (artifact_uuid, handle) via the DB index
func TestDocumentRepo_CreateDocument_UniqueHandlePerArtifact(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-create-unique"
	const artifactUUID = "artifact-create-unique"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	repo := NewDocumentRepo(db)

	first := &model.Document{
		ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypeHowTo, Handle: "overview", DisplayName: "Overview",
		Content: []byte("# first"), CreatedBy: "alice", UpdatedBy: "alice",
	}
	if err := repo.CreateDocument(first); err != nil {
		t.Fatalf("first CreateDocument: %v", err)
	}

	dup := &model.Document{
		ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypeHowTo, Handle: "overview", DisplayName: "Overview second",
		Content: []byte("# dup"), CreatedBy: "bob", UpdatedBy: "bob",
	}
	err := repo.CreateDocument(dup)
	if err == nil {
		t.Fatal("second CreateDocument with duplicate (artifact, handle) succeeded — unique index not enforced")
	}
	// The repo exposes an IsUniqueViolation helper used by UpsertDocument; the
	// service relies on it being correctly classified rather than a generic
	// error.
	if !IsUniqueViolation(err) {
		t.Errorf("second CreateDocument err = %v, want IsUniqueViolation(err) == true", err)
	}
}

func TestDocumentRepo_DocumentHandleExistsForArtifact_ScopesToArtifact(t *testing.T) {
	// The real UNIQUE index is (artifact_uuid, handle), not (org, handle) —
	// two different artifacts in the same org can both have a doc called
	// "overview". The exists-check must therefore be scoped to the artifact,
	// or create flows would reject legitimate new docs.
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-handle-exists"
	seedTestArtifact(t, db, orgUUID, "artifact-A")
	seedTestArtifact(t, db, orgUUID+"-x", "artifact-B") // separate org so FK on artifact FK target is distinct

	insertDocumentRow(t, db, &model.Document{
		ID: "docA", ArtifactUUID: "artifact-A", OrganizationUUID: orgUUID,
		Type: constants.DocumentTypeHowTo, Handle: "overview",
		DisplayName: "Overview on A",
	})

	repo := NewDocumentRepo(db)

	exists, err := repo.DocumentHandleExistsForArtifact("artifact-A", "overview")
	if err != nil {
		t.Fatalf("DocumentHandleExistsForArtifact: %v", err)
	}
	if !exists {
		t.Fatal("expected handle to exist on artifact-A")
	}

	exists, err = repo.DocumentHandleExistsForArtifact("artifact-B", "overview")
	if err != nil {
		t.Fatalf("DocumentHandleExistsForArtifact (other artifact): %v", err)
	}
	if exists {
		t.Fatal("exists-check leaked across artifacts — second artifact should be free to use the same handle")
	}
}

// DeleteApiDocument removes a how_to doc but not definition or thumbnail type docs.
func TestDocumentRepo_DeleteApiDocument_RemovesUserDocNotReserved(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-del-user"
	const artifactUUID = "artifact-del-user"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	insertDocumentRow(t, db, &model.Document{
		ID: "doc-user", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypeHowTo, Handle: "overview", DisplayName: "Overview",
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-thumb", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail,
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-def", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypeDefinition,
		Handle:      constants.DocumentHandleDefinition,
		DisplayName: constants.DocumentDisplayNameDefinition,
	})

	repo := NewDocumentRepo(db)

	if err := repo.DeleteApiDocument(artifactUUID, "overview", orgUUID); err != nil {
		t.Fatalf("DeleteApiDocument(user doc): %v", err)
	}

	for _, handle := range []string{constants.DocumentHandleThumbnail, constants.DocumentHandleDefinition} {
		err := repo.DeleteApiDocument(artifactUUID, handle, orgUUID)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("DeleteApiDocument(reserved handle %q) = %v, want sql.ErrNoRows", handle, err)
		}
	}

	var survivors int
	if err := db.QueryRow(`SELECT COUNT(*) FROM api_documents WHERE artifact_uuid = ?`, artifactUUID).Scan(&survivors); err != nil {
		t.Fatalf("count survivors: %v", err)
	}
	if survivors != 2 {
		t.Errorf("expected 2 reserved document rows to survive, found %d", survivors)
	}
}

// DeleteDocument removes definition or thumbnail type docs — the strict
// (handle, type) match is the opt-in path used by /openapi + /thumbnail.
func TestDocumentRepo_DeleteDocument_RemovesReservedDoc(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-del-reserved"
	const artifactUUID = "artifact-del-reserved"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	insertDocumentRow(t, db, &model.Document{
		ID: "doc-thumb", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail,
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-def", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypeDefinition,
		Handle:      constants.DocumentHandleDefinition,
		DisplayName: constants.DocumentDisplayNameDefinition,
	})

	repo := NewDocumentRepo(db)

	if err := repo.DeleteDocument(artifactUUID, constants.DocumentHandleThumbnail, orgUUID, constants.DocumentTypeThumbnail); err != nil {
		t.Fatalf("DeleteDocument(thumbnail): %v", err)
	}
	if err := repo.DeleteDocument(artifactUUID, constants.DocumentHandleDefinition, orgUUID, constants.DocumentTypeDefinition); err != nil {
		t.Fatalf("DeleteDocument(definition): %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM api_documents WHERE artifact_uuid = ?`, artifactUUID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected all reserved rows removed, found %d", count)
	}
}

// DeleteApiDocument throws sql.ErrNoRows if the row doesn't exist
func TestDocumentRepo_DeleteApiDocument_ReturnsErrNoRowsWhenMissing(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-del-user-missing"
	const artifactUUID = "artifact-del-user-missing"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	repo := NewDocumentRepo(db)
	err := repo.DeleteApiDocument(artifactUUID, "nonexistent", orgUUID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("DeleteApiDocument(missing) = %v, want sql.ErrNoRows", err)
	}
}

// DeleteDocument throws sql.ErrNoRows if the row doesn't exist
func TestDocumentRepo_DeleteDocument_ReturnsErrNoRowsWhenMissing(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-del-reserved-missing"
	const artifactUUID = "artifact-del-reserved-missing"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	repo := NewDocumentRepo(db)
	err := repo.DeleteDocument(artifactUUID, constants.DocumentHandleThumbnail, orgUUID, constants.DocumentTypeThumbnail)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("DeleteDocument(missing) = %v, want sql.ErrNoRows", err)
	}
}

// CreateDocument creates any type of doc and returns its UUID.
// The reserved-type guard is a service-layer concern; the repo accepts any
// well-formed row, so test every type path here (one subtest per type).
func TestDocumentRepo_CreateDocument_CreatesAnyTypeAndAssignsUUID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-create-any"
	const artifactUUID = "artifact-create-any"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	repo := NewDocumentRepo(db)

	cases := []struct {
		name    string
		docType string
		handle  string
	}{
		{"HOW_TO", constants.DocumentTypeHowTo, "overview"},
		{"THUMBNAIL", constants.DocumentTypeThumbnail, constants.DocumentHandleThumbnail},
		{"DEFINITION", constants.DocumentTypeDefinition, constants.DocumentHandleDefinition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := &model.Document{
				// Leave ID empty — the repo must generate one.
				ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
				Type: tc.docType, Handle: tc.handle,
				DisplayName: tc.name + " doc",
				Content:     []byte("content"),
				CreatedBy:   "alice", UpdatedBy: "alice",
			}
			if err := repo.CreateDocument(doc); err != nil {
				t.Fatalf("CreateDocument: %v", err)
			}
			if doc.ID == "" {
				t.Fatal("CreateDocument did not populate doc.ID")
			}
			var persisted string
			if err := db.QueryRow(`SELECT uuid FROM api_documents WHERE uuid = ?`, doc.ID).Scan(&persisted); err != nil {
				t.Fatalf("look up newly created row: %v", err)
			}
			if persisted != doc.ID {
				t.Errorf("stored uuid = %q, want %q", persisted, doc.ID)
			}
		})
	}
}

// GetDocument with docType not empty returns the doc of only that type.
func TestDocumentRepo_GetDocument_WithDocType_ReturnsOnlyThatType(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-get-type"
	const artifactUUID = "artifact-get-type"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	insertDocumentRow(t, db, &model.Document{
		ID: "doc-thumb", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail,
	})

	repo := NewDocumentRepo(db)

	doc, err := repo.GetDocument(artifactUUID, constants.DocumentHandleThumbnail, orgUUID, constants.DocumentTypeThumbnail)
	if err != nil {
		t.Fatalf("GetDocument(matching type): %v", err)
	}
	if doc == nil {
		t.Fatal("GetDocument(matching type) returned nil")
	}
	if doc.Type != constants.DocumentTypeThumbnail {
		t.Errorf("doc.Type = %q, want %q", doc.Type, constants.DocumentTypeThumbnail)
	}

	doc, err = repo.GetDocument(artifactUUID, constants.DocumentHandleThumbnail, orgUUID, constants.DocumentTypeHowTo)
	if err != nil {
		t.Fatalf("GetDocument(wrong type): %v", err)
	}
	if doc != nil {
		t.Errorf("GetDocument(wrong type) returned row of type %q, want nil", doc.Type)
	}
}

// GetDocument with docType empty returns a doc by excluding the reserved
// types (THUMBNAIL, DEFINITION) — the user-facing /docs/{id} path passes "".
func TestDocumentRepo_GetDocument_WithEmptyDocType_ExcludesReservedTypes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-get-empty"
	const artifactUUID = "artifact-get-empty"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	insertDocumentRow(t, db, &model.Document{
		ID: "doc-user", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypeHowTo, Handle: "overview", DisplayName: "Overview",
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-thumb", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail,
	})

	repo := NewDocumentRepo(db)

	doc, err := repo.GetDocument(artifactUUID, "overview", orgUUID, "")
	if err != nil {
		t.Fatalf("GetDocument(user handle, empty docType): %v", err)
	}
	if doc == nil {
		t.Fatal("expected user doc to be returned, got nil")
	}

	doc, err = repo.GetDocument(artifactUUID, constants.DocumentHandleThumbnail, orgUUID, "")
	if err != nil {
		t.Fatalf("GetDocument(reserved handle, empty docType): %v", err)
	}
	if doc != nil {
		t.Errorf("reserved row leaked onto empty-docType path, got %+v", doc)
	}
}

// ListDocumentsByArtifact with docType=OTHER returns docs of non-fixed and
// non-reserved types — both plain "Other" (stored as DOC_Other) and docs with
// a user-chosen custom name (stored as DOC_<name>, e.g. "DOC_FAQ").
func TestDocumentRepo_ListDocumentsByArtifact_WithOtherDocType_ExcludesFixedAndReservedTypes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-list-other"
	const artifactUUID = "artifact-list-other"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	insertDocumentRow(t, db, &model.Document{
		ID: "doc-howto", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: "DOC_HowTo", Handle: "howto", DisplayName: "HowTo",
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-sample", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: "DOC_Samples", Handle: "sample", DisplayName: "Sample",
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-thumb", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail,
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-faq", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:   constants.DocumentTypePrefix + "FAQ", // user-chosen OTHER name stored with DOC_ prefix
		Handle: "faq", DisplayName: "FAQ",
	})
	// Plain "Other" with no custom type name is stored as DOC_Other and must
	// also appear in ?type=OTHER results (regression for issue #3 fix).
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-plain-other", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:   constants.DocumentTypePrefix + constants.DocumentTypeOther, // "DOC_Other"
		Handle: "plain-other", DisplayName: "Plain Other",
	})

	repo := NewDocumentRepo(db)
	docs, total, err := repo.ListDocumentsByArtifact(artifactUUID, orgUUID, constants.DocumentTypeOther, 10, 0)
	if err != nil {
		t.Fatalf("ListDocumentsByArtifact(OTHER): %v", err)
	}
	if total != 2 || len(docs) != 2 {
		t.Fatalf("expected FAQ and DOC_Other rows (2 total), got total=%d docs=%d", total, len(docs))
	}
	handles := make(map[string]bool, len(docs))
	for _, d := range docs {
		handles[d.Handle] = true
	}
	for _, want := range []string{"faq", "plain-other"} {
		if !handles[want] {
			t.Errorf("expected handle %q in results; got %v", want, handles)
		}
	}
}

// UpsertDocument can be used to both insert or update a doc of reserved type.
func TestDocumentRepo_UpsertDocument_InsertsThenUpdatesReservedType(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-upsert"
	const artifactUUID = "artifact-upsert"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	repo := NewDocumentRepo(db)

	first := &model.Document{
		ID:           "doc-first",
		ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail,
		FileName:    "icon-v1.png", ContentType: "image/png",
		Content:   []byte("png-v1"),
		CreatedBy: "alice", UpdatedBy: "alice",
	}
	if err := repo.UpsertDocument(first); err != nil {
		t.Fatalf("UpsertDocument (insert): %v", err)
	}

	second := &model.Document{
		ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: "SHOULD NOT APPLY",
		FileName:    "icon-v2.png", ContentType: "image/png",
		Content:   []byte("png-v2"),
		UpdatedBy: "bob",
	}
	if err := repo.UpsertDocument(second); err != nil {
		t.Fatalf("UpsertDocument (update): %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM api_documents WHERE artifact_uuid = ? AND handle = ?`,
		artifactUUID, constants.DocumentHandleThumbnail).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one row after upsert-update, got %d", count)
	}

	var fileName, displayName string
	var content []byte
	if err := db.QueryRow(`SELECT display_name, file_name, content FROM api_documents WHERE artifact_uuid = ? AND handle = ?`,
		artifactUUID, constants.DocumentHandleThumbnail).Scan(&displayName, &fileName, &content); err != nil {
		t.Fatalf("read upserted row: %v", err)
	}
	if fileName != "icon-v2.png" {
		t.Errorf("file_name = %q, want icon-v2.png", fileName)
	}
	if string(content) != "png-v2" {
		t.Errorf("content = %q, want png-v2", content)
	}
	if displayName != constants.DocumentDisplayNameThumbnail {
		t.Errorf("display_name = %q, want unchanged %q — upsert SET list must not mutate display_name",
			displayName, constants.DocumentDisplayNameThumbnail)
	}
}

// UpdateApiDocument can be used to update a doc of non-reserved type, cannot
// update doc of reserved type.
func TestDocumentRepo_UpdateApiDocument_UpdatesUserRowRefusesReserved(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-update"
	const artifactUUID = "artifact-update"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	insertDocumentRow(t, db, &model.Document{
		ID: "doc-user", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypeHowTo, Handle: "overview", DisplayName: "Overview",
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-thumb", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail,
	})

	repo := NewDocumentRepo(db)

	// Non-reserved: succeeds, display_name mutates.
	userUpdate := &model.Document{
		ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypeHowTo, Handle: "overview",
		DisplayName: "Overview v2", UpdatedBy: "alice",
	}
	if err := repo.UpdateApiDocument(userUpdate, false); err != nil {
		t.Fatalf("UpdateApiDocument(user): %v", err)
	}
	var updatedName string
	if err := db.QueryRow(`SELECT display_name FROM api_documents WHERE uuid = ?`, "doc-user").Scan(&updatedName); err != nil {
		t.Fatalf("read user row after update: %v", err)
	}
	if updatedName != "Overview v2" {
		t.Errorf("user display_name = %q, want %q", updatedName, "Overview v2")
	}

	// Reserved: refused via sql.ErrNoRows, row untouched.
	reservedUpdate := &model.Document{
		ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: "SHOULD NOT APPLY", UpdatedBy: "alice",
	}
	err := repo.UpdateApiDocument(reservedUpdate, false)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateApiDocument(reserved) = %v, want sql.ErrNoRows", err)
	}
	var thumbName string
	if err := db.QueryRow(`SELECT display_name FROM api_documents WHERE uuid = ?`, "doc-thumb").Scan(&thumbName); err != nil {
		t.Fatalf("read thumbnail row after refused update: %v", err)
	}
	if thumbName != constants.DocumentDisplayNameThumbnail {
		t.Errorf("thumbnail display_name = %q, want unchanged %q", thumbName, constants.DocumentDisplayNameThumbnail)
	}
}

// A user doc that shares its display name with a THUMBNAIL/DEFINITION row
// must NOT register as a duplicate — the reserved rows live under their own
// endpoints and share the artifact's handle space.
func TestDocumentRepo_DocumentDisplayNameExistsForArtifact_ExcludesReservedRows(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-dn-reserved"
	const artifactUUID = "artifact-dn-reserved"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	// Thumbnail and definition rows both carry display names that could
	// collide with a user doc. The reserved-type filter must hide them.
	insertDocumentRow(t, db, &model.Document{
		ID: "thumb", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypeThumbnail, Handle: constants.DocumentHandleThumbnail,
		DisplayName: "Guide", Content: []byte{0x89},
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "def", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypeDefinition, Handle: constants.DocumentHandleDefinition,
		DisplayName: "Guide", Content: []byte("{}"),
	})

	repo := NewDocumentRepo(db)
	exists, err := repo.DocumentDisplayNameExistsForArtifact(artifactUUID, "Guide", "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if exists {
		t.Error("display name must not conflict with reserved-type rows")
	}
}

func TestDocumentRepo_DocumentDisplayNameExistsForArtifact_UserRowConflicts(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-dn-user"
	const artifactUUID = "artifact-dn-user"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	insertDocumentRow(t, db, &model.Document{
		ID: "existing", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypePrefix + constants.DocumentTypeHowTo, Handle: "existing",
		DisplayName: "Guide", Content: []byte("# existing"),
	})

	repo := NewDocumentRepo(db)
	exists, err := repo.DocumentDisplayNameExistsForArtifact(artifactUUID, "Guide", "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !exists {
		t.Error("display name 'Guide' is in use by the user doc 'existing' — must register as a duplicate")
	}
}

// excludeHandle is how a rename / in-place update excludes its OWN row from the
// uniqueness check: a doc keeping its display name on an update must not read
// as a self-conflict.
func TestDocumentRepo_DocumentDisplayNameExistsForArtifact_ExcludeHandleSkipsSelf(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-dn-exclude"
	const artifactUUID = "artifact-dn-exclude"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	insertDocumentRow(t, db, &model.Document{
		ID: "self", ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypePrefix + constants.DocumentTypeHowTo, Handle: "self",
		DisplayName: "Guide", Content: []byte("# self"),
	})

	repo := NewDocumentRepo(db)
	// Without exclude: the row DOES register as a duplicate.
	exists, err := repo.DocumentDisplayNameExistsForArtifact(artifactUUID, "Guide", "")
	if err != nil {
		t.Fatalf("err (no exclude) = %v", err)
	}
	if !exists {
		t.Error("without exclude, the own row must still appear as a conflict")
	}
	// With exclude: the row is skipped — no false self-conflict on update.
	exists, err = repo.DocumentDisplayNameExistsForArtifact(artifactUUID, "Guide", "self")
	if err != nil {
		t.Fatalf("err (with exclude) = %v", err)
	}
	if exists {
		t.Error("the own row must be skipped when excludeHandle is set")
	}
}

func TestDocumentRepo_DocumentDisplayNameExistsForArtifact_UnknownNameIsFalse(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-dn-missing"
	const artifactUUID = "artifact-dn-missing"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	repo := NewDocumentRepo(db)
	exists, err := repo.DocumentDisplayNameExistsForArtifact(artifactUUID, "Nothing", "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if exists {
		t.Error("no row carries 'Nothing' — must return false, not forward the ErrNoRows")
	}
}

// ---------------------------------------------------------------------------
// GetDocumentUUIDsByHandles — handle → uuid, scoped to artifact
// ---------------------------------------------------------------------------

func TestDocumentRepo_GetDocumentUUIDsByHandles_ResolvesHandlesWithinArtifact(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-u2h"
	const artifactA = "artifact-u2h-A"
	const artifactB = "artifact-u2h-B"
	// First call seeds org + project + artifact; second artifact under the
	// same org only needs its own project + artifact row (seedTestArtifact
	// would re-insert the org and trip the organizations.handle unique index).
	seedTestArtifact(t, db, orgUUID, artifactA)
	seedExtraArtifact(t, db, orgUUID, artifactB)

	// Both artifacts have a doc under handle "overview": the mapping must be
	// scoped per artifact (artifact_uuid, handle is the real unique index),
	// so a lookup against A must not return B's uuid.
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-A", ArtifactUUID: artifactA, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypePrefix + constants.DocumentTypeHowTo, Handle: "overview",
		DisplayName: "Overview", Content: []byte("# A"),
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-B", ArtifactUUID: artifactB, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypePrefix + constants.DocumentTypeHowTo, Handle: "overview",
		DisplayName: "Overview", Content: []byte("# B"),
	})
	// And a second handle under A, so the function has more than one row to
	// return and the test actually covers the loop.
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-A-faq", ArtifactUUID: artifactA, OrganizationUUID: orgUUID,
		Type: constants.DocumentTypePrefix + "FAQ", Handle: "faq",
		DisplayName: "FAQ", Content: []byte("# faq"),
	})

	repo := NewDocumentRepo(db)
	m, err := repo.GetDocumentUUIDsByHandles(artifactA, []string{"overview", "faq", "missing"}, orgUUID)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if m["overview"] != "doc-A" {
		t.Errorf("overview = %q, want doc-A (artifact B's uuid must NOT leak)", m["overview"])
	}
	if m["faq"] != "doc-A-faq" {
		t.Errorf("faq = %q, want doc-A-faq", m["faq"])
	}
	if _, ok := m["missing"]; ok {
		t.Errorf("missing handle must not appear in result; got %v", m)
	}
}

func TestDocumentRepo_GetDocumentUUIDsByHandles_EmptyInputReturnsEmptyMap(t *testing.T) {
	// Guard against the SQL `IN ()` degenerate case: callers batch up handles
	// and may legitimately pass an empty slice. The function must short-circuit
	// before running a query against zero placeholders.
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	repo := NewDocumentRepo(db)

	m, err := repo.GetDocumentUUIDsByHandles("any", nil, "any-org")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if m == nil || len(m) != 0 {
		t.Errorf("result = %+v, want an empty (non-nil) map", m)
	}
}

// ---------------------------------------------------------------------------
// GetDocumentHandlesByUUIDs — the inverse direction
// ---------------------------------------------------------------------------

func TestDocumentRepo_GetDocumentHandlesByUUIDs_ScopedByOrganization(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	// Two orgs, each with a doc; the function must only resolve uuids that
	// belong to the requested organization, not every row that happens to
	// carry the uuid.
	const orgA = "org-h2u-A"
	const orgB = "org-h2u-B"
	seedTestArtifact(t, db, orgA, "artifact-h2u-A")
	seedTestArtifact(t, db, orgB, "artifact-h2u-B")

	insertDocumentRow(t, db, &model.Document{
		ID: "doc-A", ArtifactUUID: "artifact-h2u-A", OrganizationUUID: orgA,
		Type: constants.DocumentTypePrefix + constants.DocumentTypeHowTo, Handle: "overview-a",
		DisplayName: "Overview A", Content: []byte("# a"),
	})
	insertDocumentRow(t, db, &model.Document{
		ID: "doc-B", ArtifactUUID: "artifact-h2u-B", OrganizationUUID: orgB,
		Type: constants.DocumentTypePrefix + constants.DocumentTypeHowTo, Handle: "overview-b",
		DisplayName: "Overview B", Content: []byte("# b"),
	})

	repo := NewDocumentRepo(db)
	m, err := repo.GetDocumentHandlesByUUIDs([]string{"doc-A", "doc-B", "missing"}, orgA)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if m["doc-A"] != "overview-a" {
		t.Errorf("doc-A = %q, want overview-a", m["doc-A"])
	}
	if _, ok := m["doc-B"]; ok {
		t.Errorf("doc-B must not leak into another org's lookup; got %v", m)
	}
	if _, ok := m["missing"]; ok {
		t.Errorf("missing uuid must not appear in result; got %v", m)
	}
}

func TestDocumentRepo_GetDocumentHandlesByUUIDs_EmptyInputReturnsEmptyMap(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	repo := NewDocumentRepo(db)

	m, err := repo.GetDocumentHandlesByUUIDs(nil, "any-org")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if m == nil || len(m) != 0 {
		t.Errorf("result = %+v, want an empty (non-nil) map", m)
	}
}

// ---------------------------------------------------------------------------
// UpsertDocument — the UPDATE branch
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// sortedMapKeys — pure helper
// ---------------------------------------------------------------------------

// sortedMapKeys returns the keys of a set-shaped `map[string]bool` in sorted
// order. Direct test — same package, no DB needed.
func TestSortedMapKeys(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]bool
		want []string
	}{
		{"empty map", map[string]bool{}, []string{}},
		{"single key", map[string]bool{"a": true}, []string{"a"}},
		{"keys returned in sorted order regardless of insertion", map[string]bool{"c": true, "a": true, "b": true}, []string{"a", "b", "c"}},
		{"false-valued keys still appear (set membership is key presence, not value)", map[string]bool{"x": false, "a": true}, []string{"a", "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sortedMapKeys(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len(result) = %d, want %d; got %v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("result[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// Existing tests cover the INSERT branch of UpsertDocument via
// UpsertDocument_InsertsThenUpdatesReservedType. This one targets a
// user-type row to pin the generic UPDATE branch and its deliberate narrow
// mutation set.
//
// UpsertDocument's UPDATE path only writes file_name, content_type, content,
// updated_by and updated_at — NOT display_name and NOT created_by. That is
// the right contract for a thumbnail-style upsert (display name is a fixed
// label; the author of record stays whoever uploaded it first). The test
// pins all four halves so a future "convenience" edit of the SET clause can't
// silently start overwriting display_name on upsert.
func TestDocumentRepo_UpsertDocument_UpdateNarrowMutation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const orgUUID = "org-upsert-update"
	const artifactUUID = "artifact-upsert-update"
	seedTestArtifact(t, db, orgUUID, artifactUUID)

	repo := NewDocumentRepo(db)

	// First upsert → INSERT branch (no existing row).
	first := &model.Document{
		ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypePrefix + constants.DocumentTypeHowTo,
		Handle:      "guide",
		DisplayName: "Original Display Name",
		FileName:    "first.md",
		ContentType: "text/markdown; charset=utf-8",
		Content:     []byte("# first"),
		CreatedBy:   "alice",
		UpdatedBy:   "alice",
	}
	if err := repo.UpsertDocument(first); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Second upsert on the same (artifact_uuid, handle, type) → UPDATE branch.
	// Every field the function MIGHT touch is given a new value so the
	// assertions below can tell exactly what the SQL writes and what it leaves.
	second := &model.Document{
		ArtifactUUID: artifactUUID, OrganizationUUID: orgUUID,
		Type:        constants.DocumentTypePrefix + constants.DocumentTypeHowTo,
		Handle:      "guide",
		DisplayName: "Changed Display Name", // Expected IGNORED on UPDATE.
		FileName:    "second.md",
		ContentType: "text/plain",
		Content:     []byte("# second"),
		CreatedBy:   "bob", // Expected IGNORED on UPDATE: created_by is immutable.
		UpdatedBy:   "bob",
	}
	if err := repo.UpsertDocument(second); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := repo.GetDocument(artifactUUID, "guide", orgUUID, "")
	if err != nil || got == nil {
		t.Fatalf("GetDocument after update: %v %+v", err, got)
	}
	// Fields that UpsertDocument's UPDATE clause writes:
	if string(got.Content) != "# second" {
		t.Errorf("content = %q, want '# second' — UPDATE must replace the body", got.Content)
	}
	if got.FileName != "second.md" {
		t.Errorf("file_name = %q, want second.md", got.FileName)
	}
	if got.ContentType != "text/plain" {
		t.Errorf("content_type = %q, want text/plain", got.ContentType)
	}
	if got.UpdatedBy != "bob" {
		t.Errorf("updated_by = %q, want bob", got.UpdatedBy)
	}
	// Fields the UPDATE clause deliberately omits:
	if got.DisplayName != "Original Display Name" {
		t.Errorf("display_name = %q, want unchanged — UPSERT UPDATE must not overwrite display_name", got.DisplayName)
	}
	if got.CreatedBy != "alice" {
		t.Errorf("created_by = %q, want unchanged alice — UPSERT UPDATE must not overwrite created_by", got.CreatedBy)
	}
}
