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

export { getApiProxy } from './apiProxies/apiProxyClient';
export {
  createApiKey,
  listApiKeys,
  revokeApiKey,
} from './apiKeys/apiKeyClient';
export {
  createApi,
  deleteApi,
  getApi,
  getApiDetail,
  listApis,
  updateApi,
} from './apis/apiClient';
export { listDeployments } from './deployments/deploymentClient';
export {
  deleteGatewayDeployment,
  deployApi,
  listGatewayDeployments,
  restoreGatewayDeployment,
  undeployGatewayDeployment,
} from './deployments/gatewayDeploymentClient';
export {
  createGateway,
  createGatewayToken,
  getGateway,
  listGateways,
} from './gateways/gatewayClient';
export {
  createDevPortal,
  deleteDevPortal,
  listDevPortals,
} from './devportal/devPortalClient';
export { listEnvironments } from './environments/environmentClient';
export {
  getOrganization,
  listOrganizations,
} from './organizations/organizationClient';
export {
  createProject,
  deleteProject,
  getProject,
  listProjects,
} from './projects/projectClient';
