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
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// PortalPublisher pushes a publication to its API Portal, addressing the
// listing by the API's own handle; no portal-returned ID is stored locally.
// HTTPPortalPublisher is the implementation.
type PortalPublisher interface {
	// Publish pushes pub, identified to the portal by apiHandle and listed as
	// apiType (e.g. rest-api, graphql-api), together with
	// its definition content (nil if none stored). A nil error means the portal
	// accepted the push. A *PortalConflictError means the portal rejected it and
	// will keep rejecting it (409 PUBLICATION_PORTAL_CONFLICT, never retried);
	// any other error is treated as unavailable (503 PUBLICATION_PORTAL_UNAVAILABLE).
	Publish(ctx context.Context, portal *model.APIPortal, apiType, apiHandle string, pub *model.Publication, definition *model.PublicationContent) error

	// Unpublish removes apiHandle's listing from portal. A nil error means the
	// portal no longer carries the listing, including when it was already gone.
	// Errors follow the same contract as Publish.
	Unpublish(ctx context.Context, portal *model.APIPortal, apiType, apiHandle string) error

	// SyncPlans makes the portal's copy of each plan in sync.Plans (matched by
	// exact handle) match the plan here:
	//   - A plan the portal lacks is created there with its pricing from
	//     sync.Pricing, since the portal takes a plan's pricing only when it
	//     creates the plan. The meters in sync.Meters that its prices charge for
	//     and that the portal lacks are created first.
	//   - A copy whose name or request limit differs is updated. A plan here
	//     changes those only while nothing offers or uses it, so such a copy is
	//     on no API.
	//   - A plan copy priced differently, or a meter with other settings, cannot
	//     be changed: nothing is written, and a *PortalPlanPricingConflictError
	//     or *PortalMeterConflictError is returned.
	//
	// When a later write fails, what this sync created is removed again. The
	// result names what it created, for UndoPlanSync. Other errors follow the
	// same contract as Publish.
	SyncPlans(ctx context.Context, portal *model.APIPortal, sync PlanSync) (PlanSyncResult, error)

	// UndoPlanSync removes from the portal what a SyncPlans created: its plans,
	// then its meters. One the portal already removed is not an error. One it
	// refuses to remove (a plan an API offers, a meter a plan prices) is left,
	// and the error says so.
	UndoPlanSync(ctx context.Context, portal *model.APIPortal, created PlanSyncResult) error

	// PublishContent adds the files in content to apiHandle's listing on portal, replacing
	// any with the same path; nothing else is removed. Errors follow the same contract as
	// Publish.
	PublishContent(ctx context.Context, portal *model.APIPortal, apiType, apiHandle string, content *PortalContent) error

	// Deprecate marks apiHandle's listing on portal as deprecated, re-sending live
	// with only the status changed. Errors follow the same contract as Publish.
	Deprecate(ctx context.Context, portal *model.APIPortal, apiType, apiHandle string, live *model.Publication) error
}

// PortalContent is what PublishContent pushes: a ZIP in the layout the portal reads.
type PortalContent struct {
	Archive []byte
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

// PlanSync is what a publish needs the API Portal to have: the plans the listing
// offers, the pricing of the paid ones (by plan UUID; a plan without an entry is
// free), and the meters made here that their prices charge for (by handle).
type PlanSync struct {
	Plans   []*model.SubscriptionPlan
	Pricing map[string]*model.SubscriptionPlanPricing
	Meters  map[string]*model.Meter
}

// PlanSyncResult names the plans and meters (by handle) a SyncPlans created in
// the API Portal, so that a publish whose listing push then fails can remove
// them again.
type PlanSyncResult struct {
	Plans  []string
	Meters []string
}

// PortalMeterConflictError signals that the API Portal already has a meter with
// this Handle, with settings that differ from the meter a published plan charges
// for. A meter a plan prices cannot change, so publishing the plan there needs a
// meter with another id.
type PortalMeterConflictError struct {
	Handle string
}

func (e *PortalMeterConflictError) Error() string {
	return fmt.Sprintf("the API Portal has a meter %s with different settings", e.Handle)
}

// PortalPlanPricingConflictError signals that the API Portal already has a plan
// with this Handle, priced differently from the plan being published. The portal
// cannot change a plan's pricing, so publishing it there needs a new handle.
type PortalPlanPricingConflictError struct {
	Handle string
}

func (e *PortalPlanPricingConflictError) Error() string {
	return fmt.Sprintf("the API Portal has a plan %s with different pricing", e.Handle)
}
