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
