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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
)

// artifactLevelGatewayID is the sentinel gateway_id used for artifact-level (non-deployed) refs.
// These rows represent the current artifact config and are used for delete protection.
const artifactLevelGatewayID = ""

// extractSecretHandles returns unique secret handles found in the given content blob.
func extractSecretHandles(content []byte) []string {
	matches := constants.SecretPlaceholderRe.FindAllSubmatch(content, -1)
	seen := make(map[string]struct{}, len(matches))
	handles := make([]string, 0, len(matches))
	for _, m := range matches {
		h := string(m[1])
		if _, ok := seen[h]; !ok {
			seen[h] = struct{}{}
			handles = append(handles, h)
		}
	}
	return handles
}

// upsertArtifactSecretRefs replaces the artifact-level (gateway_id='') refs for the given
// artifact inside an existing transaction. Call this on artifact create/update.
func upsertArtifactSecretRefs(tx *sql.Tx, db *database.DB, orgID, artifactUUID string, configJSON []byte) error {
	_, err := tx.Exec(db.Rebind(`
		DELETE FROM artifact_secret_refs
		WHERE organization_uuid = ? AND artifact_uuid = ? AND gateway_id = ?
	`), orgID, artifactUUID, artifactLevelGatewayID)
	if err != nil {
		return err
	}
	for _, handle := range extractSecretHandles(configJSON) {
		_, err = tx.Exec(db.Rebind(`
			INSERT INTO artifact_secret_refs (organization_uuid, artifact_uuid, secret_handle, gateway_id)
			VALUES (?, ?, ?, ?)
		`), orgID, artifactUUID, handle, artifactLevelGatewayID)
		if err != nil {
			return err
		}
	}
	return nil
}

// upsertDeploymentSecretRefs replaces the gateway-specific refs for an artifact on a given
// gateway inside an existing transaction. Call this on deploy; pass nil content on undeploy.
func upsertDeploymentSecretRefs(tx *sql.Tx, db *database.DB, orgID, artifactUUID, gatewayID string, content []byte) error {
	_, err := tx.Exec(db.Rebind(`
		DELETE FROM artifact_secret_refs
		WHERE organization_uuid = ? AND artifact_uuid = ? AND gateway_id = ?
	`), orgID, artifactUUID, gatewayID)
	if err != nil {
		return err
	}
	for _, handle := range extractSecretHandles(content) {
		_, err = tx.Exec(db.Rebind(`
			INSERT INTO artifact_secret_refs (organization_uuid, artifact_uuid, secret_handle, gateway_id)
			VALUES (?, ?, ?, ?)
		`), orgID, artifactUUID, handle, gatewayID)
		if err != nil {
			return err
		}
	}
	return nil
}
