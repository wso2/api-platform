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

import type { AccessControl } from './types';

export type OpenApiSpec = Record<string, unknown>;

const HTTP_METHODS = new Set([
  'get',
  'post',
  'put',
  'delete',
  'patch',
  'head',
  'options',
  'trace',
]);

export function normalizeAccessControlMode(
  mode?: string
): 'allow_all' | 'deny_all' | '' {
  if (!mode) return '';
  const normalized = String(mode).toLowerCase().replace(/-/g, '_');
  if (normalized === 'allow_all' || normalized === 'deny_all') {
    return normalized;
  }
  return '';
}

export function buildExceptionSet(accessControl?: AccessControl): Set<string> {
  const set = new Set<string>();
  (accessControl?.exceptions ?? []).forEach((exception) => {
    const path = exception.path;
    if (!path) return;

    (exception.methods ?? []).forEach((method) => {
      const methodUpper = String(method || '').toUpperCase();
      if (!methodUpper) return;
      set.add(`${methodUpper}::${path}`);
    });
  });
  return set;
}

export function filterOpenApiSpecByAccessControl(
  spec: OpenApiSpec | null,
  accessControl?: AccessControl
): OpenApiSpec | null {
  if (!spec) return null;

  const mode = normalizeAccessControlMode(accessControl?.mode);
  const exceptionSet = buildExceptionSet(accessControl);
  if (!mode) return spec;
  // `allow_all` with no exceptions means keep all resources as-is.
  // `deny_all` with no exceptions means hide all resources.
  if (mode === 'allow_all' && exceptionSet.size === 0) return spec;

  const rootPaths = spec.paths;
  if (!rootPaths || typeof rootPaths !== 'object') return spec;

  const filteredPaths: Record<string, unknown> = {};

  Object.entries(rootPaths as Record<string, unknown>).forEach(
    ([path, pathItemUnknown]) => {
      if (!pathItemUnknown || typeof pathItemUnknown !== 'object') return;

      const pathItem = pathItemUnknown as Record<string, unknown>;
      const filteredPathItem: Record<string, unknown> = {};
      let hasIncludedMethod = false;

      Object.entries(pathItem).forEach(([methodKey, operation]) => {
        const normalizedMethod = methodKey.toLowerCase();
        if (!HTTP_METHODS.has(normalizedMethod)) {
          filteredPathItem[methodKey] = operation;
          return;
        }

        const methodUpper = normalizedMethod.toUpperCase();
        const isException = exceptionSet.has(`${methodUpper}::${path}`);
        const shouldInclude = mode === 'allow_all' ? !isException : isException;

        if (shouldInclude) {
          filteredPathItem[methodKey] = operation;
          hasIncludedMethod = true;
        }
      });

      if (hasIncludedMethod) {
        filteredPaths[path] = filteredPathItem;
      }
    }
  );

  return {
    ...spec,
    paths: filteredPaths,
  };
}
