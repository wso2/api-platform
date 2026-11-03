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

import type { FetcherParams } from '@graphiql/toolkit';
import { Kind, parse } from 'graphql';

/**
 * Drops an `operationName` that doesn't name an operation in `query`.
 *
 * GraphiQL's query editor only ever updates its selected operation name when
 * the edited document has a *named* operation (`@graphiql/react`'s
 * operation-editor `handleChange` skips `setOperationName` for a falsy name),
 * so replacing GraphqlTestConsolePage's starter `query Schema { ... }` with an
 * anonymous `{ ... }` keeps sending `operationName: "Schema"` (a tab restored
 * from storage can carry a stale name the same way). A spec-compliant server —
 * and the gateway's graphql-authz policy — rejects that with "Unknown
 * operation named". An unparseable document is passed through unchanged so
 * the server reports the real syntax error.
 */
export function withValidOperationName(params: FetcherParams): FetcherParams {
  if (!params.operationName) return params;
  let names: string[];
  try {
    names = parse(params.query).definitions.flatMap((definition) =>
      definition.kind === Kind.OPERATION_DEFINITION && definition.name
        ? [definition.name.value]
        : [],
    );
  } catch {
    return params;
  }
  if (names.includes(params.operationName)) return params;
  const rest = { ...params };
  delete rest.operationName;
  return rest;
}
