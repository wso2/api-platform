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

// PortalPublisher pushes a publication to its API Portal: an existence check
// decides create vs update, both addressed by the API's own handle — no
// portal-returned reference ID is stored locally. HTTPPortalPublisher
// (http_portal_publisher.go) implements the existence check and the
// metadata+definition push for real. The content ZIP (thumbnail,
// landing-page, docs) is not yet implemented there — see that file's doc
// comment.
type PortalPublisher interface {
	// Publish pushes pub (identified to the portal by apiHandle — this API's
	// own handle, which is also api-portal's own handle/referenceId for the
	// listing) to portal, together with its definition content (nil if none
	// stored). A nil error means the portal accepted the push. A
	// *PortalConflictError means the portal rejected it and will keep
	// rejecting it (mapped to 409 PUBLICATION_PORTAL_CONFLICT, never
	// retried); any other error is treated as transient/unavailable (503
	// PUBLICATION_PORTAL_UNAVAILABLE).
	Publish(ctx context.Context, portal *model.APIPortal, apiHandle string, pub *model.Publication, definition *model.PublicationContent) error

	// Unpublish removes apiHandle's listing from portal. A nil error means
	// the portal no longer carries the listing — including the case where it
	// was already gone, which the implementation must treat as success so a
	// retry after an already-successful removal doesn't surface as an error.
	// A *PortalConflictError means the portal rejected removal and will keep
	// rejecting it as-is (e.g. active subscriptions/API keys still attached
	// — mapped to 409 PUBLICATION_PORTAL_CONFLICT, never retried); any other
	// error is treated as transient/unavailable (503
	// PUBLICATION_PORTAL_UNAVAILABLE).
	Unpublish(ctx context.Context, portal *model.APIPortal, apiHandle string) error

	// Deprecate marks apiHandle's listing on portal as deprecated, re-sending live with
	// only the status changed. Errors follow the same contract as Publish.
	Deprecate(ctx context.Context, portal *model.APIPortal, apiHandle string, live *model.Publication) error
}

// PortalConflictError signals that the API Portal rejected a publish and
// will keep rejecting it as-is — e.g. a conflicting handle or display name —
// distinct from a transient failure. See PortalPublisher.Publish.
type PortalConflictError struct {
	Message string
	// Reason is a short, pre-approved phrase describing why, resolved from a
	// closed allowlist of known portal error codes — never the portal's own
	// raw error text (error-handling.md: never expose raw downstream errors
	// to the client). Empty when the portal's response doesn't match a known
	// code; callers fall back to a generic reason in that case. Passed as
	// the %s in apperror.APIPublicationPortalConflict's message.
	Reason string
}

func (e *PortalConflictError) Error() string { return e.Message }
