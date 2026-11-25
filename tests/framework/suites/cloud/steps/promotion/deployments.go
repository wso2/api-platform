/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package promotion

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

// deployedTimeout bounds how long a deployment on a gateway may take to report DEPLOYED.
const deployedTimeout = 10 * time.Minute

// waitDeployedOnTargets waits until every deployment the last request created is DEPLOYED.
func (s *Steps) waitDeployedOnTargets(ctx context.Context) error {
	targets, err := cloudbase.RecordedTargets(ctx)
	if err != nil {
		return err
	}
	apiID, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIID)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: deployedTimeout, Interval: 5 * time.Second},
		func(ctx context.Context) (string, error) {
			resp, err := s.DoAuthenticatedRaw(ctx, http.MethodGet, "/rest-apis/"+url.PathEscape(apiID)+"/deployments", nil)
			if err != nil {
				return "", retry.Transient(err)
			}
			if resp.StatusCode != http.StatusOK {
				return "", retry.Transient(cloudbase.ResponseError(resp))
			}
			items, err := cloudbase.ListItems(resp)
			if err != nil {
				return "", retry.Transient(fmt.Errorf("decoding APIP deployments: %w", err))
			}
			status := map[string]string{}
			for _, item := range items {
				deploymentID, _ := item["deploymentId"].(string)
				state, _ := item["status"].(string)
				status[deploymentID] = state
			}
			pending := make([]string, 0, len(targets))
			for gatewayID, deploymentID := range targets {
				if status[deploymentID] != "DEPLOYED" {
					pending = append(pending, gatewayID+": "+status[deploymentID])
				}
			}
			sort.Strings(pending)
			return strings.Join(pending, ", "), nil
		},
		func(pending string) bool { return pending == "" },
		"waiting for every requested deployment to be DEPLOYED")
}

// assertBuildCount waits until the synthetic REST API holds exactly the given number of builds.
func (s *Steps) assertBuildCount(ctx context.Context, want int) error {
	return retry.Await(ctx, retry.Options{Timeout: cloudbase.PropagationTimeout(s.Topo), Interval: 3 * time.Second},
		s.FetchBuilds,
		func(items []map[string]any) bool { return len(items) == want },
		fmt.Sprintf("waiting for the synthetic REST API to hold %d builds", want))
}
