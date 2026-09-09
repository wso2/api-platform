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
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// DocumentRepo persists documents (e.g. OpenAPI specs) attached to artifacts.
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
		INSERT INTO documents
			(uuid, artifact_uuid, organization_uuid, type, handle, display_name, file_name, content, data_version, created_by, created_at, updated_by, updated_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	_, err := r.db.Exec(query,
		doc.ID, doc.ArtifactUUID, doc.OrganizationUUID, doc.Type,
		doc.Handle, doc.DisplayName, doc.FileName, doc.Content,
		doc.DataVersion, doc.CreatedBy, now, doc.CreatedBy, now,
	)
	if err != nil {
		return fmt.Errorf("create document: %w", err)
	}
	return nil
}

// GetDocumentByArtifactAndHandle retrieves a single document by artifact UUID and handle.
func (r *DocumentRepo) GetDocumentByArtifactAndHandle(artifactUUID, handle, orgUUID string) (*model.Document, error) {
	query := r.db.Rebind(`
		SELECT uuid, artifact_uuid, organization_uuid, type, handle, display_name,
		       COALESCE(file_name, ''), content, data_version,
		       COALESCE(created_by, ''), COALESCE(updated_by, '')
		FROM documents
		WHERE artifact_uuid = ? AND handle = ? AND organization_uuid = ?
	`)
	row := r.db.QueryRow(query, artifactUUID, handle, orgUUID)
	doc := &model.Document{}
	if err := row.Scan(
		&doc.ID, &doc.ArtifactUUID, &doc.OrganizationUUID, &doc.Type,
		&doc.Handle, &doc.DisplayName, &doc.FileName, &doc.Content,
		&doc.DataVersion, &doc.CreatedBy, &doc.UpdatedBy,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get document by artifact and handle: %w", err)
	}
	return doc, nil
}

// UpsertDocument inserts a new document row or updates the content/filename if one already
// exists for the same (artifact_uuid, handle) pair.
func (r *DocumentRepo) UpsertDocument(doc *model.Document) error {
	existing, err := r.GetDocumentByArtifactAndHandle(doc.ArtifactUUID, doc.Handle, doc.OrganizationUUID)
	if err != nil {
		return err
	}
	if existing == nil {
		return r.CreateDocument(doc)
	}

	now := time.Now().UTC()
	query := r.db.Rebind(`
		UPDATE documents
		SET file_name = ?, content = ?, data_version = ?, updated_by = ?, updated_at = ?
		WHERE artifact_uuid = ? AND handle = ? AND organization_uuid = ?
	`)
	_, err = r.db.Exec(query,
		doc.FileName, doc.Content, doc.DataVersion, doc.UpdatedBy, now,
		doc.ArtifactUUID, doc.Handle, doc.OrganizationUUID,
	)
	if err != nil {
		return fmt.Errorf("upsert document: %w", err)
	}
	return nil
}

// DeleteDocument removes a document by artifact UUID, handle, and org.
func (r *DocumentRepo) DeleteDocument(artifactUUID, handle, orgUUID string) error {
	query := r.db.Rebind(`DELETE FROM documents WHERE artifact_uuid = ? AND handle = ? AND organization_uuid = ?`)
	_, err := r.db.Exec(query, artifactUUID, handle, orgUUID)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	return nil
}

// DocumentHandleExistsForArtifact returns true if a document with the given handle
// already exists for the artifact, regardless of org.
func (r *DocumentRepo) DocumentHandleExistsForArtifact(artifactUUID, handle string) (bool, error) {
	query := r.db.Rebind(`SELECT 1 FROM documents WHERE artifact_uuid = ? AND handle = ? LIMIT 1`)
	row := r.db.QueryRow(query, artifactUUID, handle)
	var exists int
	if err := row.Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check document handle exists: %w", err)
	}
	return true, nil
}
