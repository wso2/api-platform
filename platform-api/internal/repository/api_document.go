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
	"fmt"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/database"
)

// ApiDocumentRepo implements ApiDocumentRepository against the api_documents
// stub table (owned by another team — see schema.*.sql). Reading/serving a
// document's own content is that team's feature; this repository only ever
// resolves a handle to the internal doc_uuid, and back.
type ApiDocumentRepo struct {
	db *database.DB
}

// NewApiDocumentRepo creates a new API document repository.
func NewApiDocumentRepo(db *database.DB) ApiDocumentRepository {
	return &ApiDocumentRepo{db: db}
}

// GetUUIDsByHandles resolves each handle to its doc_uuid, scoped to the
// organization. A handle absent from the returned map does not exist. Returns
// an empty map for empty input.
func (r *ApiDocumentRepo) GetUUIDsByHandles(handles []string, orgUUID string) (map[string]string, error) {
	if len(handles) == 0 {
		return map[string]string{}, nil
	}
	placeholders := make([]string, len(handles))
	args := make([]interface{}, 0, len(handles)+1)
	for i, h := range handles {
		placeholders[i] = "?"
		args = append(args, h)
	}
	args = append(args, orgUUID)
	query := fmt.Sprintf(`
		SELECT handle, uuid
		FROM api_documents
		WHERE handle IN (%s) AND organization_uuid = ?
	`, strings.Join(placeholders, ","))
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve document handles: %w", err)
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var handle, uuid string
		if err := rows.Scan(&handle, &uuid); err != nil {
			return nil, err
		}
		m[handle] = uuid
	}
	return m, rows.Err()
}

// GetHandlesByUUIDs is the inverse of GetUUIDsByHandles, for reconstructing a
// docIds response from stored api_publication_doc_mappings rows. Returns an
// empty map for empty input.
func (r *ApiDocumentRepo) GetHandlesByUUIDs(docUUIDs []string, orgUUID string) (map[string]string, error) {
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
		var uuid, handle string
		if err := rows.Scan(&uuid, &handle); err != nil {
			return nil, err
		}
		m[uuid] = handle
	}
	return m, rows.Err()
}
