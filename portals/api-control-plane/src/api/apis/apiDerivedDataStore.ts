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

import type { ApiProxy, Deployment, Environment } from '../../types/domain';
import { toApiProxy, toDeployment, toEnvironment } from '../adapters';
import { asArray, asRecord } from '../shared/apiClientUtils';

const apiProxiesByApi = new Map<string, ApiProxy>();
const deploymentsByApi = new Map<string, Deployment[]>();
const environmentsById = new Map<string, Environment>();

export const recordApiDerivedData = (value: unknown) => {
  const component = asRecord(value);
  const componentId = String(component.id || '');
  if (!componentId) return;

  const apiVersions = asArray(component.apiVersions);
  const latestApiVersion =
    apiVersions.find((item) => asRecord(item).latest === true) || apiVersions[0];
  const apiVersion = asRecord(latestApiVersion);
  if (Object.keys(apiVersion).length > 0) {
    apiProxiesByApi.set(
      componentId,
      toApiProxy({
        id: apiVersion.proxyId || apiVersion.id || `${componentId}-api`,
        componentId,
        context: apiVersion.proxyUrl || apiVersion.proxyName || `/${component.name}`,
        version: apiVersion.apiVersion || component.version,
        visibility: apiVersion.accessibility === 'PRIVATE' ? 'PRIVATE' : 'PUBLIC',
      })
    );
  }

  const appEnvVersions = apiVersions.flatMap((item) =>
    asArray(asRecord(item).appEnvVersions)
  );
  const deployments = appEnvVersions.map((item) => {
    const appEnvVersion = asRecord(item);
    const release = asRecord(appEnvVersion.release);
    const metadata = asRecord(release.metadata);
    const environmentId = String(
      appEnvVersion.environmentId ||
        release.environmentId ||
        release.environment ||
        ''
    );
    if (environmentId) {
      environmentsById.set(
        environmentId,
        toEnvironment({
          id: environmentId,
          name: metadata.choreoEnv || release.environment || environmentId,
          type: String(metadata.choreoEnv || release.environment || '')
            .toLowerCase()
            .includes('prod')
            ? 'PRODUCTION'
            : 'DEVELOPMENT',
        })
      );
    }
    return toDeployment({
      id: appEnvVersion.releaseId || release.id || `${componentId}-${environmentId}`,
      componentId,
      environmentId,
      status: release.id ? 'READY' : 'NOT_DEPLOYED',
      version: apiVersion.apiVersion || component.version,
      updatedAt: release.updatedAt || release.createdAt,
    });
  });
  if (deployments.length > 0) {
    deploymentsByApi.set(componentId, deployments);
  }
};

export const readDerivedApiProxy = (componentId: string) =>
  apiProxiesByApi.get(componentId);

export const readDerivedDeployments = (componentId: string) =>
  deploymentsByApi.get(componentId) || [];

export const readDerivedEnvironments = () => Array.from(environmentsById.values());
