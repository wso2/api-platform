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

import { RestApi } from '../../../../../api/resources/restApis';

export type ApiTestMode = 'curl' | 'none';

export type ApiCapabilities = {
  canBuild: boolean;
  canDeploy: boolean;
  canDevelop: boolean;
  canManage: boolean;
  canTest: boolean;
  hasRuntimeLogs: boolean;
  hasUsageInsights: boolean;
  testMode: ApiTestMode;
};

export const getApiCapabilities = (component?: RestApi): ApiCapabilities => {
  if (!component) {
    return {
      canBuild: false,
      canDeploy: false,
      canDevelop: false,
      canManage: false,
      canTest: false,
      hasRuntimeLogs: false,
      hasUsageInsights: false,
      testMode: 'none',
    };
  }

  const isApiProxy = component.kind === 'RestApi';
  const isService = component.kind === 'SERVICE';
  const isWebApp = component.kind === 'WEB_APP';
  // Normalised before comparing, never compared raw: the spec documents
  // `transport` in lowercase (`["http", "https"]`) while these constants are
  // upper, so a raw `includes('HTTP')` never matched real data — which silently
  // made `canTest` false for every API and hid the whole Test menu from the
  // sidebar. Server-supplied tokens get normalised at the boundary, the same way
  // HTTP methods do elsewhere in this repo.
  const transports = (component.transport ?? []).map((value) => value.toUpperCase());
  const isHttpReachable = transports.includes('HTTP') || transports.includes('HTTPS');

  return {
    canBuild: isService || isWebApp,
    canDeploy: isApiProxy || isService || isWebApp,
    canDevelop: isApiProxy || isService,
    canManage: isApiProxy || isService || isWebApp,
    canTest: isHttpReachable && (isApiProxy || isService),
    hasRuntimeLogs: isService || isWebApp,
    hasUsageInsights: isApiProxy || isService,
    testMode: isHttpReachable ? 'curl' : 'none',
  };
};
