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
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/platform-api/internal/constants"
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

// GetDocument retrieves a single document by (artifactUUID, handle, orgUUID)
// Returns (nil, nil) when no matching row exists.
func (r *DocumentRepo) GetDocument(artifactUUID, handle, orgUUID, docType string) (*model.Document, error) {
	whereClause := `WHERE artifact_uuid = ? AND handle = ? AND organization_uuid = ?`
	args := []interface{}{artifactUUID, handle, orgUUID}
	// docType is sent when the caller wants a strict match on type (reserved-type lookups)
	if docType != "" {
		whereClause += ` AND type = ?`
		args = append(args, docType)
	// docType is empty when the request came from the user-facing /docs/{docId} path, so exclude reserved types
	} else if len(constants.ReservedAPIDocumentTypes) > 0 {
		placeholders := make([]string, len(constants.ReservedAPIDocumentTypes))
		for i, t := range constants.ReservedAPIDocumentTypes {
			placeholders[i] = "?"
			args = append(args, t)
		}
		whereClause += ` AND type NOT IN (` + strings.Join(placeholders, ", ") + `)`
	}

	query := r.db.Rebind(`
		SELECT uuid, artifact_uuid, organization_uuid, type, handle, display_name,
		       COALESCE(file_name, ''), COALESCE(content_type, ''), content,
		       COALESCE(created_by, ''), created_at,
		       COALESCE(updated_by, ''), updated_at
		FROM api_documents
		` + whereClause)
	row := r.db.QueryRow(query, args...)
	doc := &model.Document{}
	if err := row.Scan(
		&doc.ID, &doc.ArtifactUUID, &doc.OrganizationUUID, &doc.Type,
		&doc.Handle, &doc.DisplayName, &doc.FileName, &doc.ContentType, &doc.Content,
		&doc.CreatedBy, &doc.CreatedAt,
		&doc.UpdatedBy, &doc.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get document: %w", err)
	}
	return doc, nil
}

// ListDocumentsByArtifact returns user-facing documents for an artifact,
// optionally filtered by type, as metadata-only rows (no content column).
// 
// Reserved types (constants.ReservedAPIDocumentTypes — DEFINITION, THUMBNAIL)
// are excluded at the SQL layer so the returned `total` reflects the
// user-visible row count rather than every row in the table, and pagination
// stays correct even on artifacts with many reserved rows.
func (r *DocumentRepo) ListDocumentsByArtifact(artifactUUID, orgUUID, docType string, limit, offset int) ([]*model.Document, int, error) {
	whereClause := `WHERE artifact_uuid = ? AND organization_uuid = ?`
	args := []interface{}{artifactUUID, orgUUID}
	if docType != "" {
		// get all docs whose type is not a fixed type (HowTo, Samples, …) when docType=Other
		if docType == constants.DocumentTypeOther {
			fixedTypes := constants.FixedAPIDocumentStoredTypes
			placeholders := make([]string, len(fixedTypes))
			for i, t := range fixedTypes {
				placeholders[i] = "?"
				args = append(args, t)
			}
			whereClause += ` AND type NOT IN (` + strings.Join(placeholders, ", ") + `)`
		} else {
			whereClause += ` AND type = ?`
			args = append(args, constants.DocumentTypePrefix + docType)
		}
	}
	// exclude reserved types (DEFINITION, THUMBNAIL) from listing because they have dedicated endpoints
	if len(constants.ReservedAPIDocumentTypes) > 0 {
		placeholders := make([]string, len(constants.ReservedAPIDocumentTypes))
		for i, t := range constants.ReservedAPIDocumentTypes {
			placeholders[i] = "?"
			args = append(args, t)
		}
		whereClause += ` AND type NOT IN (` + strings.Join(placeholders, ", ") + `)`
	}

	countQuery := r.db.Rebind(`SELECT COUNT(*) FROM api_documents ` + whereClause)
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count documents for artifact: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	listQuery := r.db.Rebind(`
		SELECT uuid, artifact_uuid, organization_uuid, type, handle, display_name,
		       COALESCE(file_name, ''), COALESCE(content_type, ''),
		       COALESCE(created_by, ''), created_at,
		       COALESCE(updated_by, ''), updated_at
		FROM api_documents
		` + whereClause + `
		ORDER BY updated_at DESC, uuid DESC
		` + pageClause)
	listArgs := append(append([]interface{}{}, args...), pageArgs...)
	rows, err := r.db.Query(listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list documents for artifact: %w", err)
	}
	defer rows.Close()

	docs := make([]*model.Document, 0)
	for rows.Next() {
		doc := &model.Document{}
		if err := rows.Scan(
			&doc.ID, &doc.ArtifactUUID, &doc.OrganizationUUID, &doc.Type,
			&doc.Handle, &doc.DisplayName, &doc.FileName, &doc.ContentType,
			&doc.CreatedBy, &doc.CreatedAt,
			&doc.UpdatedBy, &doc.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan document row: %w", err)
		}
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate document rows: %w", err)
	}
	return docs, total, nil
}

// UpsertDocument writes the singleton OpenAPI definition for an artifact,
// scoped by (artifact_uuid, handle, type). The matching POST
// /rest-apis/{id}/openapi endpoint was deliberately removed, so the initial
// spec upload lands here too — hence the create-if-missing branch. The SET
// list is intentionally narrow (content + file_name + content_type): the
// type (DEFINITION) and display_name are fixed by convention for this
// singleton and must not be mutated through this path.
func (r *DocumentRepo) UpsertDocument(doc *model.Document) error {
	now := time.Now().UTC()
	updateQuery := r.db.Rebind(`
		UPDATE api_documents
		SET file_name = ?, content_type = ?, content = ?, updated_by = ?, updated_at = ?
		WHERE artifact_uuid = ? AND handle = ? AND type = ? AND organization_uuid = ?
	`)
	result, err := r.db.Exec(updateQuery,
		doc.FileName, doc.ContentType, doc.Content, doc.UpdatedBy, now,
		doc.ArtifactUUID, doc.Handle, doc.Type, doc.OrganizationUUID,
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
			retryResult, retryErr := r.db.Exec(updateQuery,
				doc.FileName, doc.ContentType, doc.Content, doc.UpdatedBy, now,
				doc.ArtifactUUID, doc.Handle, doc.Type, doc.OrganizationUUID,
			)
			if retryErr != nil {
				return fmt.Errorf("failed to upsert document (retry update): %w", retryErr)
			}
			retryRows, retryErr := retryResult.RowsAffected()
			if retryErr != nil {
				return fmt.Errorf("failed to upsert document (retry rows affected): %w", retryErr)
			}
			if retryRows == 0 {
				return fmt.Errorf("upsert document retry affected 0 rows — concurrent delete during insert/update race")
			}
			return nil
		}
		return fmt.Errorf("failed to upsert document (insert): %w", err)
	}
	return nil
}

// UpdateApiDocument applies metadata and/or content changes to a user-authored
// API document identified by (artifact_uuid, handle, org). Used by the
// user-facing PUT /apis/{apiType}/{apiId}/docs/{docId} path to update either
// the content or the metadata of an existing user doc.
//
// Reserved types (DEFINITION, THUMBNAIL) are excluded from the WHERE clause so
// this method can never mutate a system-managed row — reserved rows have their
// own write path (UpsertDocument).
func (r *DocumentRepo) UpdateApiDocument(doc *model.Document, updateContent bool) error {
	now := time.Now().UTC()

	reservedPlaceholders := make([]string, len(constants.ReservedAPIDocumentTypes))
	for i := range constants.ReservedAPIDocumentTypes {
		reservedPlaceholders[i] = "?"
	}
	notReserved := ` AND type NOT IN (` + strings.Join(reservedPlaceholders, ", ") + `)`

	var (
		query  string
		result sql.Result
		err    error
	)
	if updateContent {
		query = r.db.Rebind(`
			UPDATE api_documents
			SET type = ?, display_name = ?, file_name = ?, content_type = ?, content = ?,
			    updated_by = ?, updated_at = ?
			WHERE artifact_uuid = ? AND handle = ? AND organization_uuid = ?` + notReserved)
		args := []interface{}{
			doc.Type, doc.DisplayName, doc.FileName, doc.ContentType, doc.Content,
			doc.UpdatedBy, now,
			doc.ArtifactUUID, doc.Handle, doc.OrganizationUUID,
		}
		for _, t := range constants.ReservedAPIDocumentTypes {
			args = append(args, t)
		}
		result, err = r.db.Exec(query, args...)
	} else {
		query = r.db.Rebind(`
			UPDATE api_documents
			SET type = ?, display_name = ?, file_name = ?,
			    updated_by = ?, updated_at = ?
			WHERE artifact_uuid = ? AND handle = ? AND organization_uuid = ?` + notReserved)
		args := []interface{}{
			doc.Type, doc.DisplayName, doc.FileName,
			doc.UpdatedBy, now,
			doc.ArtifactUUID, doc.Handle, doc.OrganizationUUID,
		}
		for _, t := range constants.ReservedAPIDocumentTypes {
			args = append(args, t)
		}
		result, err = r.db.Exec(query, args...)
	}
	if err != nil {
		return fmt.Errorf("failed to update document fields: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read update affected rows: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteDocument removes any type of document.
func (r *DocumentRepo) DeleteDocument(artifactUUID, handle, orgUUID, docType string) error {
	query := r.db.Rebind(`DELETE FROM api_documents
		WHERE artifact_uuid = ? AND handle = ? AND organization_uuid = ? AND type = ?`)
	result, err := r.db.Exec(query, artifactUUID, handle, orgUUID, docType)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read delete affected rows: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteApiDocument removes a document by artifact UUID, handle, and org.
// Reserved types (DEFINITION, THUMBNAIL) are excluded from the WHERE clause so
// this method can never delete a system-managed row
func (r *DocumentRepo) DeleteApiDocument(artifactUUID, handle, orgUUID string) error {
	reservedPlaceholders := make([]string, len(constants.ReservedAPIDocumentTypes))
	for i := range constants.ReservedAPIDocumentTypes {
		reservedPlaceholders[i] = "?"
	}
	query := r.db.Rebind(`DELETE FROM api_documents WHERE artifact_uuid = ? AND handle = ? AND organization_uuid = ?
		AND type NOT IN (` + strings.Join(reservedPlaceholders, ", ") + `)`)
	args := []interface{}{artifactUUID, handle, orgUUID}
	for _, t := range constants.ReservedAPIDocumentTypes {
		args = append(args, t)
	}
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read delete affected rows: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
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

// DocumentDisplayNameExistsForArtifact returns true if any user-authored document
// attached to the artifact already uses displayName
func (r *DocumentRepo) DocumentDisplayNameExistsForArtifact(artifactUUID, displayName, excludeHandle string) (bool, error) {
	args := []interface{}{artifactUUID, displayName}
	for _, t := range constants.ReservedAPIDocumentTypes {
		args = append(args, t)
	}
	reservedPlaceholders := make([]string, len(constants.ReservedAPIDocumentTypes))
	for i := range constants.ReservedAPIDocumentTypes {
		reservedPlaceholders[i] = "?"
	}
	q := `SELECT 1 FROM api_documents WHERE artifact_uuid = ? AND display_name = ?
		AND type NOT IN (` + strings.Join(reservedPlaceholders, ", ") + `)`
	if excludeHandle != "" {
		q += ` AND handle != ?`
		args = append(args, excludeHandle)
	}
	row := r.db.QueryRow(r.db.Rebind(q), args...)
	var exists int
	if err := row.Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check document display name for the artifact: %w", err)
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

func sortedMapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}