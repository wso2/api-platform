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
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// DocumentRepo persists api_documents (e.g. OpenAPI specs) attached to artifacts.
type DocumentRepo struct {
	db *database.DB
}

// NewDocumentRepo creates a new DocumentRepo.
func NewDocumentRepo(db *database.DB) DocumentRepository {
	return &DocumentRepo{db: db}
}

// CreateDocument inserts a new document row.
func (r *DocumentRepo) CreateDocument(doc *model.Document) error {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	query := r.db.Rebind(`
		INSERT INTO api_documents
			(uuid, artifact_uuid, organization_uuid, type, handle, display_name, file_name, content_type, content, created_by, created_at, updated_by, updated_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	_, err := r.db.Exec(query,
		doc.ID, doc.ArtifactUUID, doc.OrganizationUUID, doc.Type,
		doc.Handle, doc.DisplayName, doc.FileName, doc.ContentType, doc.Content,
		doc.CreatedBy, now, doc.CreatedBy, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create document: %w", err)
	}
	return nil
}

// GetDocumentByArtifactAndHandle retrieves a single document by artifact UUID and handle.
func (r *DocumentRepo) GetDocumentByArtifactAndHandle(artifactUUID, handle, orgUUID string) (*model.Document, error) {
	query := r.db.Rebind(`
		SELECT uuid, artifact_uuid, organization_uuid, type, handle, display_name,
		       COALESCE(file_name, ''), COALESCE(content_type, ''), content,
		       COALESCE(created_by, ''), COALESCE(updated_by, '')
		FROM api_documents
		WHERE artifact_uuid = ? AND handle = ? AND organization_uuid = ?
	`)
	row := r.db.QueryRow(query, artifactUUID, handle, orgUUID)
	doc := &model.Document{}
	if err := row.Scan(
		&doc.ID, &doc.ArtifactUUID, &doc.OrganizationUUID, &doc.Type,
		&doc.Handle, &doc.DisplayName, &doc.FileName, &doc.ContentType, &doc.Content,
		&doc.CreatedBy, &doc.UpdatedBy,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get document by artifact and handle: %w", err)
	}
	return doc, nil
}

// GetDocumentByArtifactAndType retrieves the single document of a given type for an artifact.
// Returns nil (no error) when no matching row exists.
func (r *DocumentRepo) GetDocumentByArtifactAndType(artifactUUID, docType, orgUUID string) (*model.Document, error) {
	query := r.db.Rebind(`
		SELECT uuid, artifact_uuid, organization_uuid, type, handle, display_name,
		       COALESCE(file_name, ''), COALESCE(content_type, ''), content,
		       COALESCE(created_by, ''), COALESCE(updated_by, '')
		FROM api_documents
		WHERE artifact_uuid = ? AND type = ? AND organization_uuid = ?
	`)
	row := r.db.QueryRow(query, artifactUUID, docType, orgUUID)
	doc := &model.Document{}
	if err := row.Scan(
		&doc.ID, &doc.ArtifactUUID, &doc.OrganizationUUID, &doc.Type,
		&doc.Handle, &doc.DisplayName, &doc.FileName, &doc.ContentType, &doc.Content,
		&doc.CreatedBy, &doc.UpdatedBy,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get document by artifact and type: %w", err)
	}
	return doc, nil
}

// UpsertDocument inserts or updates a document for the given (artifact_uuid, handle) pair.
func (r *DocumentRepo) UpsertDocument(doc *model.Document) error {
	now := time.Now().UTC()
	updateQuery := r.db.Rebind(`
		UPDATE api_documents
		SET file_name = ?, content_type = ?, content = ?, updated_by = ?, updated_at = ?
		WHERE artifact_uuid = ? AND handle = ? AND organization_uuid = ?
	`)
	result, err := r.db.Exec(updateQuery,
		doc.FileName, doc.ContentType, doc.Content, doc.UpdatedBy, now,
		doc.ArtifactUUID, doc.Handle, doc.OrganizationUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert document (update): %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to upsert document (rows affected): %w", err)
	}
	if rows > 0 {
		return nil
	}

	// No existing row — try to insert.
	if err := r.CreateDocument(doc); err != nil {
		if IsUniqueViolation(err) {
			// A concurrent writer inserted between our UPDATE and INSERT; retry the UPDATE.
			_, err = r.db.Exec(updateQuery,
				doc.FileName, doc.ContentType, doc.Content, doc.UpdatedBy, now,
				doc.ArtifactUUID, doc.Handle, doc.OrganizationUUID,
			)
			if err != nil {
				return fmt.Errorf("failed to upsert document (retry update): %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to upsert document (insert): %w", err)
	}
	return nil
}

// DeleteDocument removes a document by artifact UUID, handle, and org.
func (r *DocumentRepo) DeleteDocument(artifactUUID, handle, orgUUID string) error {
	query := r.db.Rebind(`DELETE FROM api_documents WHERE artifact_uuid = ? AND handle = ? AND organization_uuid = ?`)
	_, err := r.db.Exec(query, artifactUUID, handle, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	return nil
}

// DocumentHandleExistsForArtifact returns true if a document with the given handle
// already exists for the artifact, regardless of org.
func (r *DocumentRepo) DocumentHandleExistsForArtifact(artifactUUID, handle string) (bool, error) {
	// No LIMIT clause: QueryRow returns at most one row, and the unique constraint
	// on (artifact_uuid, handle) guarantees at most one exists. LIMIT 1 is
	// also incompatible with SQL Server syntax.
	query := r.db.Rebind(`SELECT 1 FROM api_documents WHERE artifact_uuid = ? AND handle = ?`)
	row := r.db.QueryRow(query, artifactUUID, handle)
	var exists int
	if err := row.Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check document handle for the artifact: %w", err)
	}
	return true, nil
}

// GetDocumentUUIDsByHandles resolves each handle to its document uuid,
// scoped to one artifact — api_documents' real unique index is
// (artifact_uuid, handle), so a handle is only guaranteed unique per
// artifact, not per org. Returns an empty map for empty input.
func (r *DocumentRepo) GetDocumentUUIDsByHandles(artifactUUID string, handles []string, orgUUID string) (map[string]string, error) {
	if len(handles) == 0 {
		return map[string]string{}, nil
	}
	placeholders := make([]string, len(handles))
	args := make([]interface{}, 0, len(handles)+2)
	for i, h := range handles {
		placeholders[i] = "?"
		args = append(args, h)
	}
	args = append(args, artifactUUID, orgUUID)
	query := fmt.Sprintf(`
		SELECT handle, uuid
		FROM api_documents
		WHERE handle IN (%s) AND artifact_uuid = ? AND organization_uuid = ?
	`, strings.Join(placeholders, ","))
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve document handles: %w", err)
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var handle, id string
		if err := rows.Scan(&handle, &id); err != nil {
			return nil, err
		}
		m[handle] = id
	}
	return m, rows.Err()
}

// GetDocumentHandlesByUUIDs is the inverse of GetDocumentUUIDsByHandles, for
// reconstructing a docIds response from stored api_publication_doc_mappings
// rows. Scoped to the organization only — a mapping row's doc_uuid already
// came from that same artifact's own resolved set. Returns an empty map for
// empty input.
func (r *DocumentRepo) GetDocumentHandlesByUUIDs(docUUIDs []string, orgUUID string) (map[string]string, error) {
	if len(docUUIDs) == 0 {
		return map[string]string{}, nil
	}
	placeholders := make([]string, len(docUUIDs))
	args := make([]interface{}, 0, len(docUUIDs)+1)
	for i, id := range docUUIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, orgUUID)
	query := fmt.Sprintf(`
		SELECT uuid, handle
		FROM api_documents
		WHERE uuid IN (%s) AND organization_uuid = ?
	`, strings.Join(placeholders, ","))
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve document uuids: %w", err)
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var id, handle string
		if err := rows.Scan(&id, &handle); err != nil {
			return nil, err
		}
		m[id] = handle
	}
	return m, rows.Err()
}
