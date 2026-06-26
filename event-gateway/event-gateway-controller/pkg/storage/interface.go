/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package storage

import "github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"

// EventStorage handles webhook secret persistence for event-gateway-controller.
// Implemented by EventSQLStore using the *sql.DB from Services.Storage.GetDB().
type EventStorage interface {
	// Initialize creates the webhook_secrets table if it does not yet exist.
	Initialize() error
	SaveWebhookSecret(secret *models.WebhookSecret) error
	GetWebhookSecretsByArtifact(artifactUUID string) ([]*models.WebhookSecret, error)
	GetWebhookSecretByArtifactAndName(artifactUUID, name string) (*models.WebhookSecret, error)
	GetWebhookSecretByUUID(uuid string) (*models.WebhookSecret, error)
	UpdateWebhookSecret(secret *models.WebhookSecret) error
	DeleteWebhookSecret(artifactUUID, name string) error
	GetAllWebhookSecrets() ([]*models.WebhookSecret, error)
}
