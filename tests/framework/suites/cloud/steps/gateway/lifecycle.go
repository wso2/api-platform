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

package gateway

import (
	"context"
	"fmt"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

// SelectOtherGatewayEnvironment selects a listed non-production environment other than the active
// gateway's, for a gateway the runner creates beside the one it creates there. An environment a
// suite run once created for itself is passed over.
func (s *Steps) SelectOtherGatewayEnvironment(ctx context.Context) error {
	active, err := cloudbase.RequiredValue(ctx, cloudbase.KeyGatewayEnv)
	if err != nil {
		return fmt.Errorf("the active gateway does not report its environment: %w", err)
	}
	items, err := s.ListEnvironments(ctx)
	if err != nil {
		return err
	}
	name, err := cloudbase.OtherEnvironment(items, active, cloudbase.HostsSuiteGateways(ctx, s.RawDo))
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, cloudbase.KeyEnvironmentName, name)
}
