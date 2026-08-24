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

import type { Schema } from '../../api/core/spec';

/**
 * Fixture builders for platform-api entities.
 *
 * Every return type is the **generated spec type**, which is the point: a
 * fixture that does not match the contract fails to compile rather than
 * producing a test that passes against a shape the server never sends. Two real
 * bugs were caught this way — `upstream.production` (the field is `main`) and
 * `CreateRESTAPIRequest` resolving to an uninhabitable type.
 *
 * Each builder fills only the required fields and takes an override object, so
 * a test states the one or two values it actually cares about:
 *
 *     aRestApi({ id: 'pizza-shack', lifeCycleStatus: 'PUBLISHED' })
 */

export type RestApiFixture = Schema<'RESTAPI'>;
export type ProjectFixture = Schema<'Project'>;
export type OrganizationFixture = Schema<'Organization'>;
export type GatewayFixture = Schema<'GatewayResponse'>;
export type DeploymentFixture = Schema<'DeploymentResponse'>;

export const aRestApi = (
  overrides: Partial<RestApiFixture> = {}
): RestApiFixture => ({
  id: 'pizza-shack',
  displayName: 'Pizza Shack',
  context: '/pizza',
  version: 'v1',
  projectId: 'retail',
  kind: 'REST',
  lifeCycleStatus: 'PUBLISHED',
  upstream: { main: { url: 'https://upstream.test' } },
  ...overrides,
});

export const aProject = (
  overrides: Partial<ProjectFixture> = {}
): ProjectFixture => ({
  id: 'retail',
  displayName: 'Retail APIs',
  organizationId: 'acme-org',
  ...overrides,
});

export const anOrganization = (
  overrides: Partial<OrganizationFixture> = {}
): OrganizationFixture => ({
  id: 'acme-org',
  displayName: 'Acme Corp',
  region: 'us-east-1',
  ...overrides,
});

export const aGateway = (
  overrides: Partial<GatewayFixture> = {}
): GatewayFixture => ({
  id: 'shared-gateway',
  displayName: 'Shared Gateway',
  endpoints: ['https://gw.test'],
  functionalityType: 'regular',
  isCritical: false,
  isActive: true,
  version: '1.0.0',
  ...overrides,
});

/** Body for creating a gateway, which requires more than the response carries. */
export const aCreateGatewayBody = (
  overrides: Partial<Schema<'CreateGatewayRequest'>> = {}
): Schema<'CreateGatewayRequest'> => ({
  displayName: 'Edge Gateway',
  endpoints: ['https://edge.test'],
  functionalityType: 'regular',
  isCritical: false,
  version: '1.0.0',
  ...overrides,
});

/** Body for deploying an API; `name` and `base` are required alongside the gateway. */
export const aDeployRequest = (
  overrides: Partial<Schema<'DeployRequest'>> = {}
): Schema<'DeployRequest'> => ({
  name: 'pizza-shack-v1',
  base: 'v1',
  gatewayId: 'shared-gateway',
  ...overrides,
});

export const aDeployment = (
  overrides: Partial<DeploymentFixture> = {}
): DeploymentFixture => ({
  deploymentId: 'deployment-1',
  name: 'pizza-shack on shared-gateway',
  gatewayId: 'shared-gateway',
  status: 'DEPLOYED',
  createdAt: '2026-01-01T00:00:00Z',
  ...overrides,
});

export type ApplicationFixture = Schema<'Application'>;
export type SubscriptionFixture = Schema<'Subscription'>;
export type SubscriptionPlanFixture = Schema<'SubscriptionPlan'>;
export type SecretFixture = Schema<'SecretSummary'>;
export type CustomPolicyFixture = Schema<'CustomPolicyResponse'>;

export const anApplication = (
  overrides: Partial<ApplicationFixture> = {}
): ApplicationFixture => ({
  id: 'checkout-app',
  displayName: 'Checkout App',
  projectId: 'retail',
  type: 'genai',
  ...overrides,
});

export const aSubscription = (
  overrides: Partial<SubscriptionFixture> = {}
): SubscriptionFixture => ({
  id: 'subscription-1',
  artifactId: 'pizza-shack',
  applicationId: 'checkout-app',
  subscriberId: 'user-1',
  subscriptionPlanId: 'gold',
  status: 'ACTIVE',
  ...overrides,
});

export const aSubscriptionPlan = (
  overrides: Partial<SubscriptionPlanFixture> = {}
): SubscriptionPlanFixture => ({
  id: 'gold',
  displayName: 'Gold',
  status: 'ACTIVE',
  ...overrides,
});

export const aSecret = (
  overrides: Partial<SecretFixture> = {}
): SecretFixture => ({
  id: 'signing-key',
  displayName: 'Signing Key',
  type: 'GENERIC',
  status: 'ACTIVE',
  ...overrides,
});

export const aCustomPolicy = (
  overrides: Partial<CustomPolicyFixture> = {}
): CustomPolicyFixture => ({
  uuid: 'policy-uuid-1',
  organizationUuid: 'acme-org',
  name: 'rate-limit',
  version: 'v1',
  policyDefinition: {},
  ...overrides,
});

/**
 * Generates `count` sequentially-named entities, for paging tests.
 *
 *     manyRestApis(30)  // API 1 … API 30, ids api-1 … api-30
 */
export const manyRestApis = (count: number): RestApiFixture[] =>
  Array.from({ length: count }, (_unused, index) =>
    aRestApi({ id: `api-${index + 1}`, displayName: `API ${index + 1}` })
  );

export const manyProjects = (count: number): ProjectFixture[] =>
  Array.from({ length: count }, (_unused, index) =>
    aProject({ id: `project-${index + 1}`, displayName: `Project ${index + 1}` })
  );
