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

/**
 * The API's lifecycle state on one gateway. `none` is not a server state: it is
 * how a gateway with no deployment at all is rendered, so an environment can
 * show every gateway it has rather than only the deployed ones.
 */
export type DeploymentStatus =
  | 'DEPLOYED'
  | 'DEPLOYING'
  | 'UNDEPLOYED'
  | 'UNDEPLOYING'
  | 'FAILED'
  | 'ARCHIVED'
  | 'none';

/** One gateway of an environment, with the API's deployment on it. */
export type Gateway = {
  id: string;
  /** Display name where the gateway list provides one; the handle otherwise. */
  name: string;
  status: DeploymentStatus;
  deploymentId?: string;
  deploymentName?: string;
  /** Error code explaining a FAILED deployment. */
  statusReason?: string;
  deployedAt?: string;
};

/**
 * One environment of the project's deployment pipeline, in promotion order.
 * `buildId` is the deployment this environment is currently serving — what a
 * promotion out of it carries forward — and its presence is what makes the next
 * environment promotable.
 */
export type Environment = {
  name: string;
  buildId?: string;
  gateways: Gateway[];
};

/**
 * One customizable deployment setting for an environment. The catalog is served
 * by the API rather than hardcoded here, so a setting can be added without a UI
 * change; `type` drives client-side validation only.
 */
export type DeploymentParameter = {
  name: string;
  label: string;
  description: string;
  type: 'url' | 'host' | string;
  value: string;
};
