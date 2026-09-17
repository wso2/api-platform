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

// PortalPublisher pushes a publication to its API Portal (REST_Design.md §8):
// an existence check decides create vs update, both addressed by the API's
// own handle — no portal-returned reference ID is stored locally.
// HTTPPortalPublisher (http_portal_publisher.go) implements the existence
// check and the metadata+definition push (§8 steps 1-2) for real. Step 3
// (the content ZIP — thumbnail/landing-page/docs) is not yet implemented
// there — see that file's doc comment — so every caller still gets the
// stand-in below unless a real shared key is configured
// (config.PublicationPortalSharedKeyPath); it always succeeds, standing in
// for the whole push where no portal integration is configured.
type PortalPublisher interface {
	// Publish pushes pub (identified to the portal by apiHandle — this API's
	// own handle, which is also api-portal's own handle/referenceId for the
	// listing) to portal, together with its definition content (nil if none
	// stored). A nil error means the portal accepted the push. A
	// *PortalConflictError means the portal rejected it and will keep
	// rejecting it (mapped to 409 PUBLICATION_PORTAL_CONFLICT, never
	// retried); any other error is treated as transient/unavailable (503
	// PUBLICATION_PORTAL_UNAVAILABLE).
	Publish(ctx context.Context, portal *model.PublicationAPIPortal, apiHandle string, pub *model.Publication, definition *model.PublicationContent) error
}

// PortalConflictError signals that the API Portal rejected a publish and
// will keep rejecting it as-is — e.g. a conflicting handle or display name —
// distinct from a transient failure. See PortalPublisher.Publish.
type PortalConflictError struct {
	Message string
}

func (e *PortalConflictError) Error() string { return e.Message }

// standInPortalPublisher is the one mock Slice 5's minimal demo needs — its
// Publish always succeeds, standing in for the real portal push until that
// integration is built (see "When the real dependencies land" in
// Implementation_Plan.md).
type standInPortalPublisher struct{}

// NewStandInPortalPublisher returns a PortalPublisher whose Publish always
// succeeds.
func NewStandInPortalPublisher() PortalPublisher {
	return &standInPortalPublisher{}
}

func (*standInPortalPublisher) Publish(_ context.Context, _ *model.PublicationAPIPortal, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	return nil
}
