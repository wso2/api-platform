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
)

// deployToGateway deploys the current definition of a REST API to one gateway of an environment
// through the API's project and returns the id of the deployment created on that gateway.
func (s *Steps) deployToGateway(ctx context.Context, apiID, environment, gatewayID string) (string, error) {
	deployed, err := s.DeployArtifact(ctx, "", apiID, environment, gatewayID, "")
	return deployed.ID, err
}
