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
	"reflect"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func TestPlanRefConversions(t *testing.T) {
	if PlanRefsToModel(nil) != nil || PlanRefHandles(nil) != nil || AttachedPlansToRefs(nil) != nil {
		t.Fatal("a missing list stays missing")
	}
	name := "ignored"
	in := &[]api.SubscriptionPlanRef{{Id: "gold", DisplayName: &name}, {Id: "silver"}}
	m := PlanRefsToModel(in)
	if !reflect.DeepEqual(m.Handles(), []string{"gold", "silver"}) || m[0].DisplayName != "" {
		t.Fatalf("model = %+v; a request's displayName is read-only", m)
	}
	refs := AttachedPlansToRefs(model.AttachedPlans{{Handle: "gold", DisplayName: "Gold"}, {Handle: "silver"}})
	if refs == nil || len(*refs) != 2 || *(*refs)[0].DisplayName != "Gold" || (*refs)[1].DisplayName != nil {
		t.Fatalf("refs = %+v", refs)
	}
}

// The gateway still gets plan handles in the deployment, whatever form the plans are stored in.
func TestAPIDeploymentYAML_PlanHandles(t *testing.T) {
	plans := model.AttachedPlans{{Handle: "gold", DisplayName: "Gold"}, {Handle: "silver"}}
	if got := plans.Handles(); !reflect.DeepEqual(got, []string{"gold", "silver"}) {
		t.Fatalf("got %v", got)
	}
}
