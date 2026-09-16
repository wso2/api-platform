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
import type { BodyOf, PathOf, QueryOf, ResponseOf, Schema } from '../../../core/spec';

/**
 * Transport layer for `/graphql-apis/{graphqlApiId}/deployments`.
 *
 * Deployments are a sub-resource, rooted at one GraphQL API — same shape as
 * `restApis/deployments`, and deliberately its own module rather than a
 * shared generic: the spec declares this as its own operation family
 * (`DeployGraphQLAPI`, `GetGraphQLAPIDeployments`, …), even though it reuses
 * REST's `DeployRequest`/`DeploymentResponse` schemas verbatim.
 */

export type Deployment = Schema<'DeploymentResponse'>;
export type DeploymentListResponse = ResponseOf<'GetGraphQLAPIDeployments'>;
export type ListDeploymentsQuery = QueryOf<'GetGraphQLAPIDeployments'>;
export type DeployGraphQLApiBody = BodyOf<'DeployGraphQLAPI'>;

/** Lifecycle states a deployment can be in. */
export type DeploymentStatus = Deployment['status'];

const collectionPath = (
  graphqlApiId: PathOf<'GetGraphQLAPIDeployments'>['graphqlApiId'],
): string => `/graphql-apis/${encodeURIComponent(graphqlApiId)}/deployments`;

const resourcePath = (
  graphqlApiId: string,
  deploymentId: PathOf<'GetGraphQLAPIDeployment'>['deploymentId'],
): string => `${collectionPath(graphqlApiId)}/${encodeURIComponent(deploymentId)}`;

export const listDeployments = async (
  graphqlApiId: string,
  options?: RequestOptions,
): Promise<DeploymentListResponse> => {
  return http.get<DeploymentListResponse>(collectionPath(graphqlApiId), {
    ...options,
    operationName: 'GetGraphQLAPIDeployments',
  });
};

export const getDeployment = async (
  graphqlApiId: string,
  deploymentId: string,
  options?: RequestOptions,
): Promise<Deployment> => {
  return http.get<Deployment>(resourcePath(graphqlApiId, deploymentId), {
    ...options,
    operationName: 'GetGraphQLAPIDeployment',
  });
};

/**
 * Deploys an API to a gateway. Returns immediately with the deployment in a
 * transitional state — the gateway acknowledges asynchronously, so callers
 * poll the collection until the status settles.
 */
export const deployApi = async (
  graphqlApiId: string,
  body: DeployGraphQLApiBody,
  options?: RequestOptions,
): Promise<Deployment> => {
  return http.post<Deployment>(collectionPath(graphqlApiId), body, {
    ...options,
    operationName: 'DeployGraphQLAPI',
  });
};

/** Takes a deployment out of service while keeping its record. */
export const undeployDeployment = async (
  graphqlApiId: string,
  deploymentId: string,
  options?: RequestOptions,
): Promise<Deployment> => {
  return http.post<Deployment>(
    `${resourcePath(graphqlApiId, deploymentId)}/undeploy`,
    undefined,
    { ...options, operationName: 'UndeployGraphQLAPIDeployment' },
  );
};

/** Returns a previously undeployed deployment to service. */
export const restoreDeployment = async (
  graphqlApiId: string,
  deploymentId: string,
  options?: RequestOptions,
): Promise<Deployment> => {
  return http.post<Deployment>(
    `${resourcePath(graphqlApiId, deploymentId)}/restore`,
    undefined,
    { ...options, operationName: 'RestoreGraphQLAPIDeployment' },
  );
};

/** Removes the deployment record entirely. Irreversible. */
export const deleteDeployment = async (
  graphqlApiId: string,
  deploymentId: string,
  options?: RequestOptions,
): Promise<void> => {
  await http.delete<void>(resourcePath(graphqlApiId, deploymentId), {
    ...options,
    operationName: 'DeleteGraphQLAPIDeployment',
  });
};
