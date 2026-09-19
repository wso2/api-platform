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

package service

import (
	"context"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// PortalPublisher pushes a publication to its API Portal, addressing the
// listing by the API's own handle; no portal-returned ID is stored locally.
// HTTPPortalPublisher is the implementation.
type PortalPublisher interface {
	// Publish pushes pub, identified to the portal by apiHandle, together with
	// its definition content (nil if none stored). A nil error means the portal
	// accepted the push. A *PortalConflictError means the portal rejected it and
	// will keep rejecting it (409 PUBLICATION_PORTAL_CONFLICT, never retried);
	// any other error is treated as unavailable (503 PUBLICATION_PORTAL_UNAVAILABLE).
	Publish(ctx context.Context, portal *model.APIPortal, apiHandle string, pub *model.Publication, definition *model.PublicationContent) error

	// Unpublish removes apiHandle's listing from portal. A nil error means the
	// portal no longer carries the listing, including when it was already gone.
	// Errors follow the same contract as Publish.
	Unpublish(ctx context.Context, portal *model.APIPortal, apiHandle string) error

	// Deprecate marks apiHandle's listing on portal as deprecated, re-sending live
	// with only the status changed. Errors follow the same contract as Publish.
	Deprecate(ctx context.Context, portal *model.APIPortal, apiHandle string, live *model.Publication) error
}

// PortalConflictError signals that the API Portal rejected a request and will
// keep rejecting it as-is, unlike a transient failure.
type PortalConflictError struct {
	Message string
	// Reason is a short, pre-approved phrase drawn from a closed set of known
	// portal error codes, never the portal's raw error text. Empty when the code
	// is unknown; callers then use a generic reason.
	Reason string
}

func (e *PortalConflictError) Error() string { return e.Message }
