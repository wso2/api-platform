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

import (
	"context"
	"time"
)

// PortalWebhookKeys is one portal's prepared verification material. The
// receiver loads it per request from a PortalKeyProvider implementation (the
// service-layer registry) and binds the delivery's org to this portal's own
// org before dispatching. Previous* are populated only inside the rotation
// grace window; the receiver tries Current first and falls back to Previous
// when PreviousValidUntil is still in the future.
//
// Lives in the webhook package (rather than service) so the receiver can
// consume it without importing service — service imports webhook for Verifier
// and Decryptor, so the other direction would be a cycle.
type PortalWebhookKeys struct {
	PortalUUID         string
	OrganizationUUID   string
	Current            *Verifier
	CurrentDecryptor   *Decryptor
	Previous           *Verifier
	PreviousDecryptor  *Decryptor
	PreviousValidUntil time.Time
}

// HasPreviousInGrace reports whether the previous secret should still be
// accepted at `now`. A nil Previous or zero PreviousValidUntil means no
// previous secret is on file; a non-zero value in the past means the grace
// has elapsed.
func (k *PortalWebhookKeys) HasPreviousInGrace(now time.Time) bool {
	if k == nil || k.Previous == nil {
		return false
	}
	return !k.PreviousValidUntil.IsZero() && now.Before(k.PreviousValidUntil)
}

// PortalKeyProvider resolves per-portal verification material by portal uuid.
// Implemented by service.PortalWebhookKeyRegistry; declared here so the
// receiver depends only on the webhook package.
type PortalKeyProvider interface {
	Get(ctx context.Context, portalUUID string) (*PortalWebhookKeys, error)
}
