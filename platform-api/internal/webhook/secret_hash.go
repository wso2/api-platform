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

package webhook

import "github.com/wso2/api-platform/platform-api/internal/utils"

// SecretHash returns the identifier the API Portal sends as the envelope's secret_hash: the
// lowercase hex SHA3-256 of "<label>:<secret>". It names which of several valid secrets signed
// and encrypted a delivery; it is also what platform-api stores as the webhook secret's
// secret_hash, so the receiver finds the secret by an indexed lookup. One-way, not key material.
func SecretHash(secret string) string { return utils.WebhookSecretHash(secret) }
