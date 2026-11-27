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

/*
 * The API Platform console features this page reports, in the console's own
 * terms (`portals/api-control-plane/src/featureUsage/api-platform-taxonomy.yaml`):
 * the built-in Deploy page and gateway registration report the same codes and
 * dimensions, so a console usage dashboard counts a cloud deploy like any other.
 */

import type { ArtifactKind } from './deployApi';

export const API_DEPLOY_CLICK = 'apip.lifecycle.api-deploy-click';
export const DEPLOYMENT_UNDEPLOY_CLICK = 'apip.lifecycle.deployment-undeploy-click';
export const GATEWAY_REGISTER_CLICK = 'apip.gateway.gateway-register-click';
export const GATEWAY_TOKEN_REGENERATE_CLICK = 'apip.gateway.gateway-token-regenerate-click';
export const GATEWAY_SETUP_COMMAND_COPY = 'apip.gateway.gateway-setup-command-copy';
export const API_DEPLOY_OPEN = 'apip.lifecycle.api-deploy-open';
export const BUILD_DELETE_CLICK = 'apip.lifecycle.build-delete-click';

/**
 * The taxonomy's `deploy_mode`: ship an existing build, build and deploy in one
 * step, or promote the build running in the previous environment.
 */
export type DeployMode = 'deploy' | 'build-and-deploy' | 'promote';

/**
 * The taxonomy's `artifact_kind` for an artifact this page deploys. Only the
 * console's APIs have one: the AI Workspace kinds are not console features, so
 * nothing is reported for them.
 */
const ARTIFACT_KINDS: Partial<Record<ArtifactKind, 'rest-api' | 'graphql-api'>> = {
  RestApi: 'rest-api',
  GraphQLApi: 'graphql-api',
};

export const featureArtifactKind = (kind: ArtifactKind): 'rest-api' | 'graphql-api' | undefined =>
  ARTIFACT_KINDS[kind];
