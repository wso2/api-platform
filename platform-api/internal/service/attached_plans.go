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

package service

import (
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// stampAttachedPlans stores each plan with its current name and the time it is attached (now), as a
// publication stores its selected plans, so a later read can tell whether the plan changed since.
// A plan that is not in the catalog keeps only its handle. Saving the API again stamps the plans
// again, which makes an UPDATED plan UNCHANGED.
func stampAttachedPlans(planRepo repository.SubscriptionPlanRepository, plans model.AttachedPlans, orgUUID string, now time.Time) (model.AttachedPlans, error) {
	if len(plans) == 0 || planRepo == nil {
		return plans, nil
	}
	byHandle, err := planRepo.GetByHandles(plans.Handles(), orgUUID)
	if err != nil {
		return nil, err
	}
	at := now.UTC()
	stamped := make(model.AttachedPlans, 0, len(plans))
	for _, p := range plans {
		out := model.AttachedPlan{Handle: p.Handle}
		if plan, ok := byHandle[p.Handle]; ok && plan != nil {
			out.DisplayName = plan.Name
			out.AttachedAt = &at
		}
		stamped = append(stamped, out)
	}
	return stamped, nil
}

// annotatePlanRefs fills in each reference's displayName and sourceStatus against the catalog, by the
// publication rule (see sourceStatus in the publication handler): DELETED when the plan no longer
// exists (its stored name is kept), UPDATED when it changed after it was attached, else UNCHANGED.
// A plan stored before the details were kept (no attach time) reads as UNCHANGED.
func annotatePlanRefs(planRepo repository.SubscriptionPlanRepository, refs *[]api.SubscriptionPlanRef, stored model.AttachedPlans, orgUUID string) error {
	if refs == nil || len(*refs) == 0 || planRepo == nil {
		return nil
	}
	byHandle, err := planRepo.GetByHandles(stored.Handles(), orgUUID)
	if err != nil {
		return err
	}
	attachedAt := make(map[string]*time.Time, len(stored))
	for _, p := range stored {
		attachedAt[p.Handle] = p.AttachedAt
	}
	for i := range *refs {
		ref := &(*refs)[i]
		status := api.SubscriptionPlanRefUnchanged
		plan, ok := byHandle[ref.Id]
		switch {
		case !ok || plan == nil:
			status = api.SubscriptionPlanRefDeleted
		default:
			name := plan.Name
			ref.DisplayName = &name
			if at := attachedAt[ref.Id]; at != nil && plan.UpdatedAt.After(*at) {
				status = api.SubscriptionPlanRefUpdated
			}
		}
		ref.SourceStatus = &status
	}
	return nil
}

// planRefsWithHandles replaces a request's references with their (validated, trimmed) handles only:
// displayName and sourceStatus are read-only and not taken from a request.
func planRefsWithHandles(handles []string) []api.SubscriptionPlanRef {
	refs := make([]api.SubscriptionPlanRef, 0, len(handles))
	for _, h := range handles {
		refs = append(refs, api.SubscriptionPlanRef{Id: h})
	}
	return refs
}
