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

package utils

import (
	"crypto/sha3"
	"encoding/hex"
)

// webhookSecretHashLabel domain-separates the webhook secret hash from any other digest of the
// same value. It must stay byte-identical to the API Portal's label
// (portals/api-portal/src/services/webhooks/envelopeCrypto.js, SECRET_HASH_LABEL); the "-v1"
// suffix is the scheme version, bumped on both sides together.
const webhookSecretHashLabel = "api-portal-webhook-secret-hash-v1"

// WebhookSecretHash returns the identifier of a webhook secret: the lowercase hex SHA3-256 of
// "<label>:<secret>". The API Portal sends it as the delivery's secret_hash and platform-api
// stores it beside each secret version, so a receiver finds the right secret by lookup instead
// of trying every one. It is a one-way identifier, not key material.
func WebhookSecretHash(secret string) string {
	sum := sha3.Sum256([]byte(webhookSecretHashLabel + ":" + secret))
	return hex.EncodeToString(sum[:])
}
