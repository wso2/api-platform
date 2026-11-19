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

package restapi

import (
	"context"
	"net/url"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

// VerifyAPIDeleted waits until the scenario's REST API is gone and deregisters it from cleanup.
func (s *Steps) VerifyAPIDeleted(ctx context.Context) error {
	id, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIID)
	if err != nil {
		return err
	}
	if err := s.AwaitNotFound(ctx, "/rest-apis/"+url.PathEscape(id), "REST API "+id); err != nil {
		return err
	}
	cloudbase.Deregister(ctx, cleanup.KindAPI, id)
	return nil
}
