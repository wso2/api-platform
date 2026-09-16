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

import type { GraphQLApiListItem } from '@/api/resources/graphqlApis';
import type { RestApi } from '@/api/resources/restApis';

/**
 * The one field the card/row/grid views need that neither `RestApi` nor
 * `GraphQLApiListItem` carries: which endpoint (and therefore which detail
 * page and delete mutation) this row belongs to. The two already share every
 * other field `ApiCard`/`ApiListView` read (`id`, `displayName`,
 * `description`, `version`, `kind`, `createdAt`, `updatedAt`) structurally,
 * so this is the only adapter the merged list needs — no wrapper object, no
 * per-view branching.
 *
 * `GraphQLApiListItem`, not `GraphQLApi`: `GraphQLAPIListResponse.list`'s
 * items are the spec's slimmer `GraphQLAPIListItem` schema, missing fields
 * only the detail/create shape carries (`schemaSource`, `sdl`, ...). Using
 * the full `GraphQLApi` here compiles right up until a real list value is
 * passed through it, and fails in a confusing way at that call site instead.
 */
export type ApiKind = 'rest' | 'graphql';
export type ListableApi = (RestApi | GraphQLApiListItem) & { apiType: ApiKind };

export const toRestListableApi = (api: RestApi): ListableApi => ({ ...api, apiType: 'rest' });
export const toGraphQLListableApi = (api: GraphQLApiListItem): ListableApi => ({
  ...api,
  apiType: 'graphql',
});
