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

import {
  GRAPHQL_API_TYPE,
  REST_API_TYPE,
  type PublishableApiType,
} from '@/api/resources/apiPublications';
import { routes } from '@/routes/paths';

/**
 * Which API the publish pages act on, and where they link. REST and GraphQL
 * APIs share the pages; each kind supplies one of these.
 */
export type PublishTarget = {
  /** The `{apiType}` path segment the publication endpoints take. */
  apiType: PublishableApiType;
  apiHandle: string;
  /** What its definition is written in: an OpenAPI document, or a GraphQL SDL. */
  definitionKind: 'graphql' | 'openapi';
  /** The API's Portals list, which every terminal action returns to. */
  portalsPath: string;
  /** One portal's publish page for this API. */
  publishPath: (apiPortalId: string) => string;
};

/** A REST API in console scope. */
export const restPublishTarget = (
  orgHandle: string,
  projectHandler: string,
  apiHandle: string,
): PublishTarget => ({
  apiHandle,
  apiType: REST_API_TYPE,
  definitionKind: 'openapi',
  portalsPath: routes.apiPortals(orgHandle, projectHandler, apiHandle),
  publishPath: (apiPortalId) =>
    routes.apiPortalPublish(orgHandle, projectHandler, apiHandle, apiPortalId),
});

/** A GraphQL API, from its own route. */
export const graphqlPublishTarget = (
  orgHandle: string,
  projectHandler: string,
  apiHandle: string,
): PublishTarget => ({
  apiHandle,
  apiType: GRAPHQL_API_TYPE,
  definitionKind: 'graphql',
  portalsPath: routes.graphqlApiPublish(orgHandle, projectHandler, apiHandle),
  publishPath: (apiPortalId) =>
    routes.graphqlApiPortalPublish(orgHandle, projectHandler, apiHandle, apiPortalId),
});
