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
 * Reads the operation currently open in Swagger UI and the values entered into
 * its form, without wrapping any Swagger UI components.
 *
 * This restriction is intentional. The original console implementation
 * obtained live updates by wrapping Swagger UI's `OperationContainer` with
 * `wrapComponents` and injecting a reporter. Because that component controls
 * operation expansion and collapse, this is a highly invasive integration
 * point. It also cannot be reliably tested in this repository: jsdom resolves
 * Swagger UI's Node build, which includes its own React instance and does not
 * reconcile with React 19. The browser instead uses
 * `swagger-ui-es-bundle-core`. Finally, the wrapper was part of the code path
 * associated with the reported issue.
 *
 * Reading the store does not modify the rendering process. This module uses
 * published selectors or direct state reads, handles unexpected state shapes
 * defensively, and returns `undefined` rather than throwing if Swagger UI's
 * internal structures change.
 */

/** The optional parts of Swagger UI's system used by this module. */
export type SwaggerSystemLike = {
  getState?: () => unknown;
  specSelectors?: {
    operations?: () => unknown;
    parameterValues?: (path: string, method: string) => { toJS?: () => unknown } | undefined;
    requestBodyValue?: (path: string, method: string) => unknown;
    specJson?: () => { toJS?: () => unknown } | undefined;
  };
};

type ImmutableLike = {
  get?: (key: string) => unknown;
  getIn?: (path: string[]) => unknown;
  forEach?: (iteratee: (value: unknown, key: unknown) => void) => void;
  toJS?: () => unknown;
};

const asImmutable = (value: unknown): ImmutableLike | undefined =>
  typeof value === 'object' && value !== null ? (value as ImmutableLike) : undefined;

/**
 * Returns the `operationId` values that Swagger UI currently considers expanded.
 *
 * Layout state is represented as a nested map:
 * `shown.operations[tag][operationId] = true`.
 * Multiple operations may be expanded simultaneously. The order in this map
 * determines which operation the cURL view follows, so this function returns a
 * list rather than a single value.
 */
export const shownOperationIds = (system: SwaggerSystemLike): string[] => {
  const state = asImmutable(system.getState?.());
  const operations = asImmutable(state?.getIn?.(['layout', 'shown', 'operations']));
  if (!operations?.forEach) return [];

  const ids: string[] = [];
  operations.forEach((byOperation) => {
    asImmutable(byOperation)?.forEach?.((isShown, operationId) => {
      if (isShown === true && typeof operationId === 'string') ids.push(operationId);
    });
  });
  return ids;
};

/**
 * Resolves an expanded operation to its path and HTTP method.
 *
 * Swagger UI keys layout state by `operationId`, while the console uses the
 * path and HTTP method. These values are reconciled through the operation list.
 * For specifications without an `operationId`, this function also supports
 * Swagger UI's synthesized identifier: `{method}{path}` with non-word
 * characters removed, as produced by its `opId` helper.
 */
export const resolveShownOperation = (
  system: SwaggerSystemLike,
): { path: string; method: string } | undefined => {
  const shown = shownOperationIds(system);
  if (shown.length === 0) return undefined;

  const operations = asImmutable(system.specSelectors?.operations?.());
  if (!operations?.forEach) return undefined;

  const candidates: { path: string; method: string; ids: string[] }[] = [];
  operations.forEach((entry) => {
    const record = asImmutable(entry);
    const path = record?.get?.('path');
    const method = record?.get?.('method');
    if (typeof path !== 'string' || typeof method !== 'string') return;

    const operation = asImmutable(record?.get?.('operation'));
    const operationId = operation?.get?.('operationId');
    const synthesised = `${method}${path}`.replace(/[^\w]/g, '');

    candidates.push({
      ids: [
        ...(typeof operationId === 'string' ? [operationId] : []),
        synthesised,
        `${method.toLowerCase()}-${path}`,
      ],
      method,
      path,
    });
  });

  // Use the order in `shown`, rather than the order in the specification. If
  // multiple operations are expanded, the cURL view should follow the first
  // expanded operation listed by Swagger UI.
  for (const id of shown) {
    const match = candidates.find((candidate) => candidate.ids.includes(id));
    if (match) return { method: match.method, path: match.path };
  }

  return undefined;
};

/** Returns the parsed document, or `undefined` if Swagger UI has not loaded one. */
export const specJsonOf = (system: SwaggerSystemLike): Record<string, unknown> | undefined => {
  const spec = system.specSelectors?.specJson?.()?.toJS?.();
  return typeof spec === 'object' && spec !== null ? (spec as Record<string, unknown>) : undefined;
};

/** Returns the values entered into an operation's form. */
export const formValuesOf = (
  system: SwaggerSystemLike,
  path: string,
  method: string,
): { parameterValues: Record<string, unknown>; bodyValue?: string } => {
  const values = system.specSelectors?.parameterValues?.(path, method)?.toJS?.();
  const bodyValue = system.specSelectors?.requestBodyValue?.(path, method);

  return {
    bodyValue: typeof bodyValue === 'string' ? bodyValue : undefined,
    parameterValues:
      typeof values === 'object' && values !== null ? (values as Record<string, unknown>) : {},
  };
};
