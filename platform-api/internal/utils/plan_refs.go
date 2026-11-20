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

package utils

import (
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// PlanRefsToModel returns the plans a request attaches, by handle only: the service validates them
// and stores each with its name and the time it was attached. A request's displayName and
// sourceStatus are read-only and left out.
func PlanRefsToModel(refs *[]api.SubscriptionPlanRef) model.AttachedPlans {
	if refs == nil {
		return nil
	}
	return model.AttachedPlansFromHandles(PlanRefHandles(refs))
}

// PlanRefHandles returns the handles of a request's plan references, in order; nil when the request
// leaves the list out.
func PlanRefHandles(refs *[]api.SubscriptionPlanRef) []string {
	if refs == nil {
		return nil
	}
	handles := make([]string, 0, len(*refs))
	for _, ref := range *refs {
		handles = append(handles, ref.Id)
	}
	return handles
}

// AttachedPlansToRefs returns the stored plans as references with the name stored with each; the
// service replaces the name with the plan's current one and adds sourceStatus. Nil when the API
// has no plans, so the field is left out as before.
func AttachedPlansToRefs(plans model.AttachedPlans) *[]api.SubscriptionPlanRef {
	if len(plans) == 0 {
		return nil
	}
	refs := make([]api.SubscriptionPlanRef, 0, len(plans))
	for _, plan := range plans {
		refs = append(refs, api.SubscriptionPlanRef{Id: plan.Handle, DisplayName: StringPtrIfNotEmpty(plan.DisplayName)})
	}
	return &refs
}
