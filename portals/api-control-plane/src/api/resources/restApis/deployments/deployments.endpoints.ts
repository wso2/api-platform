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

import { http, type RequestOptions } from '../../../core/http';
import type {
  BodyOf,
  PathOf,
  QueryOf,
  ResponseOf,
  Schema,
} from '../../../core/spec';

/**
 * Transport layer for `/rest-apis/{restApiId}/deployments`.
 *
 * Deployments are a sub-resource: every path is rooted at one REST API, so
 * `restApiId` is a required first argument rather than a filter. The spec
 * declares parallel operations for LLM providers, LLM proxies and MCP proxies
 * under their own operation ids; those get their own modules rather than a
 * shared generic, so each stays typed by its own spec operation.
 */

export type Deployment = Schema<'DeploymentResponse'>;
export type DeploymentListResponse = ResponseOf<'GetDeployments'>;
export type ListDeploymentsQuery = QueryOf<'GetDeployments'>;
export type DeployApiBody = BodyOf<'DeployAPI'>;

/** Lifecycle states a deployment can be in. */
export type DeploymentStatus = Deployment['status'];

const collectionPath = (restApiId: PathOf<'GetDeployments'>['restApiId']): string =>
  `/rest-apis/${encodeURIComponent(restApiId)}/deployments`;

const resourcePath = (
  restApiId: string,
  deploymentId: PathOf<'GetDeployment'>['deploymentId']
): string =>
  `${collectionPath(restApiId)}/${encodeURIComponent(deploymentId)}`;

export const listDeployments = async (
  restApiId: string,
  options?: RequestOptions
): Promise<DeploymentListResponse> => {
  return http.get<DeploymentListResponse>(collectionPath(restApiId), {
    ...options,
    operationName: 'GetDeployments',
  });
};

export const getDeployment = async (
  restApiId: string,
  deploymentId: string,
  options?: RequestOptions
): Promise<Deployment> => {
  return http.get<Deployment>(resourcePath(restApiId, deploymentId), {
    ...options,
    operationName: 'GetDeployment',
  });
};

/**
 * Deploys an API to a gateway. Returns immediately with the deployment in a
 * transitional state — the gateway acknowledges asynchronously, so callers
 * poll the collection until the status settles.
 */
export const deployApi = async (
  restApiId: string,
  body: DeployApiBody,
  options?: RequestOptions
): Promise<Deployment> => {
  return http.post<Deployment>(collectionPath(restApiId), body, {
    ...options,
    operationName: 'DeployAPI',
  });
};

/** Takes a deployment out of service while keeping its record. */
export const undeployDeployment = async (
  restApiId: string,
  deploymentId: string,
  options?: RequestOptions
): Promise<Deployment> => {
  return http.post<Deployment>(
    `${resourcePath(restApiId, deploymentId)}/undeploy`,
    undefined,
    { ...options, operationName: 'UndeployDeployment' }
  );
};

/** Returns a previously undeployed deployment to service. */
export const restoreDeployment = async (
  restApiId: string,
  deploymentId: string,
  options?: RequestOptions
): Promise<Deployment> => {
  return http.post<Deployment>(
    `${resourcePath(restApiId, deploymentId)}/restore`,
    undefined,
    { ...options, operationName: 'RestoreDeployment' }
  );
};

/** Removes the deployment record entirely. Irreversible. */
export const deleteDeployment = async (
  restApiId: string,
  deploymentId: string,
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(resourcePath(restApiId, deploymentId), {
    ...options,
    operationName: 'DeleteDeployment',
  });
};
