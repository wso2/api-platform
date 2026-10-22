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

package model

import (
	"database/sql"
	"time"
)

// APIPortalWebhookSecret is one portal's webhook signing secret. The receiver route
// POST /api/internal/v0.9/webhook/portals/{portalUUID}/events keys off PortalUUID,
// verifies the HMAC with SecretEnc (decrypted), and derives the field-encryption
// key from the same secret. PreviousSecretEnc + PreviousValidUntil carry the
// rotation grace window: the receiver accepts either secret until the grace expires,
// so single-attempt deliveries signed between a rotation and the subscriber-row
// update are not lost.
type APIPortalWebhookSecret struct {
	PortalUUID         string       `db:"portal_uuid"`
	SecretEnc          []byte       `db:"secret_enc"`
	PreviousSecretEnc  []byte       `db:"previous_secret_enc"`
	PreviousValidUntil sql.NullTime `db:"previous_valid_until"`
	CreatedAt          time.Time    `db:"created_at"`
	RotatedAt          sql.NullTime `db:"rotated_at"`
}

// TableName returns the table name for the APIPortalWebhookSecret model.
func (APIPortalWebhookSecret) TableName() string {
	return "api_portal_webhook_secrets"
}
