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

import { normalizeMethod, type ConsoleRequest, type KeyValueRow } from './types';

/**
 * Builds a `ConsoleRequest` from an operation's OpenAPI definition and the
 * values entered in Swagger UI's try-out form.
 *
 * Swagger UI's `requestFor` and `mutatedRequestFor` are populated only after
 * execution, so they cannot represent the form before a request is sent.
 * Instead, this module reads the live values from `parameterValues(path,
 * method)` and `requestBodyValue(path, method)`, then maps them to the
 * operation's path, query, and header parameters.
 *
 * This keeps the integration limited to two selectors and avoids duplicating
 * Swagger UI's request pipeline or scraping its DOM. The implementation is
 * pure and operates on plain objects.
 *
 * After execution, `fromSwaggerRequest` in `swaggerRequest.ts` takes precedence
 * because the interceptor provides the authoritative request bytes.
 */

/** Parameter slots this console can fill. `cookie` is not offered. */
const SUPPORTED_LOCATIONS = new Set(['path', 'query', 'header']);

type ParameterDefinition = {
  name: string;
  in: string;
};

let rowSequence = 0;
/** Monotonic ids so two rows sharing a name stay independently editable. */
const nextRowId = (prefix: string): string => {
  rowSequence += 1;
  return `${prefix}-${rowSequence}`;
};

const row = (
  prefix: string,
  name: string,
  value: string,
  options: { secret?: boolean; auto?: boolean } = {},
): KeyValueRow => ({
  id: nextRowId(prefix),
  name,
  value,
  enabled: true,
  ...options,
});

/** A trimmed string, or `undefined` when nothing usable is left. */
const text = (value: unknown): string | undefined => {
  if (typeof value === 'string') {
    const next = value.trim();
    return next === '' ? undefined : next;
  }
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  return undefined;
};

/**
 * Reads the declared parameters of one operation, including the path-level ones
 * OpenAPI allows to be shared across every method on a path.
 *
 * Path-level parameters are easy to forget and their absence is invisible: the
 * operation simply renders without an input the spec said it takes.
 */
export const operationParameters = (
  spec: Record<string, unknown>,
  path: string,
  method: string,
): ParameterDefinition[] => {
  const paths = spec.paths;
  if (typeof paths !== 'object' || paths === null) return [];

  const pathItem = (paths as Record<string, unknown>)[path];
  if (typeof pathItem !== 'object' || pathItem === null) return [];

  const item = pathItem as Record<string, unknown>;
  const operation = item[method.toLowerCase()];
  const operationParams =
    typeof operation === 'object' && operation !== null
      ? (operation as Record<string, unknown>).parameters
      : undefined;

  const collect = (value: unknown): ParameterDefinition[] =>
    Array.isArray(value)
      ? value.flatMap((entry) => {
          if (typeof entry !== 'object' || entry === null) return [];
          const { name, in: location } = entry as Record<string, unknown>;
          if (typeof name !== 'string' || typeof location !== 'string') return [];
          if (!SUPPORTED_LOCATIONS.has(location)) return [];
          return [{ name, in: location }];
        })
      : [];

  // Operation-level parameters win over path-level ones of the same name and
  // location, per the OpenAPI spec's override rule.
  const shared = collect(item.parameters);
  const own = collect(operationParams);
  const ownKeys = new Set(own.map((parameter) => `${parameter.in}.${parameter.name}`));

  return [...shared.filter((p) => !ownKeys.has(`${p.in}.${p.name}`)), ...own];
};

/**
 * Looks a parameter's value out of swagger's parameter map.
 *
 * swagger keys the map by `paramToIdentifier`, which yields `in.name`
 * (`query.limit`). The bare name is checked as a fallback because that is the
 * other identifier form the same helper can produce, and which form appears is
 * not something this console should depend on.
 */
export const parameterValue = (
  values: Record<string, unknown>,
  parameter: ParameterDefinition,
): string | undefined =>
  text(values[`${parameter.in}.${parameter.name}`]) ?? text(values[parameter.name]);

/**
 * Substitutes path-parameter values, leaving unfilled placeholders unchanged.
 */
export const fillPathParameters = (
  path: string,
  parameters: ParameterDefinition[],
  values: Record<string, unknown>,
): string =>
  parameters
    .filter((parameter) => parameter.in === 'path')
    .reduce((filled, parameter) => {
      const value = parameterValue(values, parameter);
      if (value === undefined) return filled;
      return filled.split(`{${parameter.name}}`).join(value);
    }, path);

export type BuildConsoleRequestArgs = {
  spec: Record<string, unknown>;
  path: string;
  method: string;
  baseUrl: string;
  /** swagger's `parameterValues(path, method).toJS()`. */
  parameterValues?: Record<string, unknown>;
  /** swagger's `requestBodyValue(path, method)`, as text or a structure. */
  bodyValue?: unknown;
  /** Headers the console adds itself, e.g. the test key. */
  extraHeaders?: KeyValueRow[];
};

/** The body as text, whatever swagger stored. */
const bodyText = (value: unknown): string => {
  if (typeof value === 'string') return value;
  if (value === undefined || value === null) return '';
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return '';
  }
};

/**
 * Builds a request from the operation's populated form.
 *
 * Declared headers precede console-provided headers (`extraHeaders`), which
 * take precedence when names overlap. Content-Type is derived from the body
 * by `contentTypeFor` and emitted by `toCurl`.
 */
export const buildConsoleRequest = ({
  spec,
  path,
  method,
  baseUrl,
  parameterValues = {},
  bodyValue,
  extraHeaders = [],
}: BuildConsoleRequestArgs): ConsoleRequest => {
  const parameters = operationParameters(spec, path, method);
  const body = bodyText(bodyValue);
  const hasBodyText = body.trim() !== '';

  const declaredHeaders = parameters
    .filter((parameter) => parameter.in === 'header')
    .map((parameter) => row('h', parameter.name, parameterValue(parameterValues, parameter) ?? ''));

  const declaredNames = new Set(declaredHeaders.map((header) => header.name.toLowerCase()));

  return {
    method: normalizeMethod(method),
    baseUrl: baseUrl.trim().replace(/\/+$/, ''),
    path: fillPathParameters(path, parameters, parameterValues),
    queryParams: parameters
      .filter((parameter) => parameter.in === 'query')
      .flatMap((parameter) => {
        const value = parameterValue(parameterValues, parameter);
        // An untouched optional query parameter is omitted rather than sent
        // empty: `?status=` and no `status` at all mean different things to
        // most servers.
        return value === undefined ? [] : [row('q', parameter.name, value)];
      }),
    headers: [
      ...declaredHeaders,
      ...extraHeaders.filter((header) => !declaredNames.has(header.name.toLowerCase())),
    ],
    bodyMode: hasBodyText ? 'raw' : 'none',
    // swagger's try-out form composes JSON request bodies, so a body arriving
    // from there is JSON. A user who wants another encoding picks it in the
    // cURL view, which owns that choice.
    rawFormat: 'json',
    body,
    formFields: [],
  };
};

/** Returns the first operation in declaration order. */
export const firstOperationOf = (
  spec: Record<string, unknown>,
): { path: string; method: string } | undefined => {
  const paths = spec.paths;
  if (typeof paths !== 'object' || paths === null) return undefined;

  for (const [path, pathItem] of Object.entries(paths as Record<string, unknown>)) {
    if (typeof pathItem !== 'object' || pathItem === null) continue;
    const item = pathItem as Record<string, unknown>;
    const method = ['get', 'post', 'put', 'patch', 'delete', 'head', 'options'].find(
      (candidate) => typeof item[candidate] === 'object' && item[candidate] !== null,
    );
    if (method) return { method: method.toUpperCase(), path };
  }

  return undefined;
};
