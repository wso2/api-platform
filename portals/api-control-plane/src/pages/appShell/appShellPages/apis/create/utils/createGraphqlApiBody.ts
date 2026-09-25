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

import type { CreateGraphQLApiBody } from '@/api/resources/graphqlApis';
import type { GraphqlApiCreationFormState } from '../types';

/** What the platform calls a GraphQL API artifact. */
const GRAPHQL_API_KIND = 'GraphQLApi';

/** A trimmed value, or `undefined` when there is nothing left to send. */
const trimmed = (value: string | undefined): string | undefined => {
  const next = value?.trim();
  return next === '' ? undefined : next;
};

/**
 * Maps the GraphQL wizard's form state onto `POST /graphql-apis`'s multipart
 * body — see `createRestApiBody.ts` for why this mapping lives in its own
 * module rather than being spread from the form directly (server-owned
 * fields never travel; blank optionals are dropped, not sent empty).
 *
 * `schemaSource`/`sdl`/`sdlUrl`/`sdlFile` all come from what the source step
 * already resolved — the configure step never re-declares how the schema was
 * supplied, it only adds the identity/routing fields.
 */
export const toCreateGraphQLApiBody = (
  formState: GraphqlApiCreationFormState,
  scope: { projectId: string },
): CreateGraphQLApiBody => {
  const id = trimmed(formState.id);
  const description = trimmed(formState.description);

  // Only the field matching `schemaSource` may be present — the backend
  // rejects `sdl` alongside `sdlUrl`/`sdlFile`/`introspection` as a structural
  // mismatch (400 VALIDATION_FAILED), not a silent override. `formState.sdl`
  // is the *resolved* schema the source step already showed in the explorer;
  // for every source but `inline` the backend re-resolves it itself and this
  // field must be omitted, not forwarded.
  const schemaSourceFields: Pick<CreateGraphQLApiBody['metadata'], 'sdl' | 'sdlUrl'> =
    formState.schemaSource === 'inline'
      ? { sdl: trimmed(formState.sdl) }
      : formState.schemaSource === 'url'
        ? { sdlUrl: trimmed(formState.sdlUrl) }
        : {};

  return {
    metadata: {
      ...(id === undefined ? {} : { id }),
      displayName: formState.displayName.trim(),
      ...(description === undefined ? {} : { description }),
      context: formState.context.trim(),
      version: formState.version.trim(),
      projectId: scope.projectId,
      kind: GRAPHQL_API_KIND,
      schemaSource: formState.schemaSource,
      ...schemaSourceFields,
      upstream: { main: { url: formState.endpointUrl.trim() } },
    },
    ...(formState.schemaSource === 'file' && formState.sdlFile !== undefined
      ? { sdlFile: formState.sdlFile }
      : {}),
  };
};
