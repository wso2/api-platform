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
 * The API's deployment state on one gateway, in the API's own vocabulary so
 * nothing has to be translated on the way in. `NOT_DEPLOYED` is a real answer
 * from the server for a gateway the API has never been deployed to, which is how
 * an environment can list every gateway it has rather than only the deployed
 * ones.
 */
export type DeploymentStatus =
  | 'DEPLOYED'
  | 'DEPLOYING'
  | 'UNDEPLOYED'
  | 'UNDEPLOYING'
  | 'FAILED'
  | 'ARCHIVED'
  | 'NOT_DEPLOYED';

/**
 * Whether the gateway itself is up and able to receive a deployment. This is the
 * gateway's own health, reported by the gateways resource, and is a different
 * fact from `DeploymentStatus`: a perfectly healthy gateway has nothing deployed
 * on it until someone deploys, and a gateway that has gone away still shows the
 * deployment it last ran.
 */
export type GatewayHealth = 'active' | 'inactive';

/** One gateway of an environment, with the API's deployment on it. */
export type Gateway = {
  id: string;
  /** Display name from the gateways resource; the handle when it has none. */
  name: string;
  /** The host this gateway serves on, shown as its subtitle. */
  host?: string;
  health: GatewayHealth;
  status: DeploymentStatus;
  /** Preselected in the deploy/promote picker when an environment has several. */
  isDefault?: boolean;
  /** Needed to stop what is running; absent when nothing is. */
  deploymentId?: string;
  /** The build this gateway is running. */
  buildId?: string;
  deployedAt?: string;
  /** Backend URL this gateway was last deployed with, and the form's starting value. */
  endpointUrl?: string;
  /** Error code explaining a FAILED deployment. */
  statusReason?: string;
};

/**
 * One environment of the project's deployment pipeline, in promotion order.
 * Environments are named, not numbered: the name is what the pipeline, the
 * gateway bindings and every deploy request agree on, so there is no separate id.
 */
export type Environment = {
  name: string;
  gateways: Gateway[];
};

/**
 * An immutable snapshot of the API's definition. Deploying to the pipeline's
 * first environment prepares one and sends it; promoting carries an existing one
 * forward. The id is the date it was prepared and that day's index.
 */
export type Build = {
  buildId: string;
  /** The note recorded when the build was prepared, if any. */
  description?: string;
  createdAt?: string;
  createdBy?: string;
};
