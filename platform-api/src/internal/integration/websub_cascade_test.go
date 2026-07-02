//go:build integration && experimental

/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

// WebSub web broker integration tests. These are kept under the "experimental"
// build tag because the WebSub feature is not yet generally available.
// Run with: go test -tags "integration experimental" ./internal/integration/...

package integration

import (
	"testing"
)

// TestCascade_DeleteWebSubAPIRemovesHmacSecrets verifies that deleting the
// artifacts row that backs a WebSub API cascade-removes both the websub_apis row
// and all associated websub_api_hmac_secrets rows (two independent CASCADE edges
// from artifacts). This catches regressions if either FK is changed to NO ACTION.
func TestCascade_DeleteWebSubAPIRemovesHmacSecrets(t *testing.T) {
	it := openExperimentalITDB(t)

	orgUUID := id()
	projectUUID := id()
	artifactUUID := id()

	it.exec(t, `INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid) VALUES (?, ?, ?, ?, ?)`,
		orgUUID, "wsc-"+orgUUID[:8], "cascade org", "us", "idp-ref")
	it.exec(t, `INSERT INTO projects (uuid, handle, display_name, organization_uuid) VALUES (?, ?, ?, ?)`,
		projectUUID, "cascade-proj", "cascade-proj", orgUUID)
	it.exec(t, `INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
		artifactUUID, "WebSubApi", orgUUID)
	it.exec(t, `INSERT INTO websub_apis (uuid, organization_uuid, handle, display_name, version, project_uuid, lifecycle_status, configuration) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		artifactUUID, orgUUID, "ws-api-"+artifactUUID[:8], "ws-api", "v1.0", projectUUID, "CREATED", []byte("{}"))

	secret1 := id()
	secret2 := id()
	it.exec(t, `INSERT INTO websub_api_hmac_secrets (uuid, artifact_uuid, handle, encrypted_secret, status) VALUES (?, ?, ?, ?, ?)`,
		secret1, artifactUUID, "github-secret", []byte("enc1"), "active")
	it.exec(t, `INSERT INTO websub_api_hmac_secrets (uuid, artifact_uuid, handle, encrypted_secret, status) VALUES (?, ?, ?, ?, ?)`,
		secret2, artifactUUID, "gitlab-secret", []byte("enc2"), "active")

	if got := it.count(t, "websub_api_hmac_secrets", "artifact_uuid", artifactUUID); got != 2 {
		t.Fatalf("precondition: want 2 hmac secrets, got %d", got)
	}
	if got := it.count(t, "websub_apis", "uuid", artifactUUID); got != 1 {
		t.Fatalf("precondition: want 1 websub_api row, got %d", got)
	}

	it.exec(t, `DELETE FROM artifacts WHERE uuid = ?`, artifactUUID)

	if got := it.count(t, "websub_api_hmac_secrets", "artifact_uuid", artifactUUID); got != 0 {
		t.Fatalf("[%s] hmac secrets not cascade-deleted after artifact delete: %d remain", it.driver, got)
	}
	if got := it.count(t, "websub_apis", "uuid", artifactUUID); got != 0 {
		t.Fatalf("[%s] websub_api not cascade-deleted after artifact delete: %d remain", it.driver, got)
	}
}
