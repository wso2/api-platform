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

import type {
  ApiOperation,
  ApiPolicy,
  ApiDetail,
  HttpMethod,
} from '../../../../../types/domain';

export const HTTP_METHODS: HttpMethod[] = [
  'GET',
  'POST',
  'PUT',
  'DELETE',
  'PATCH',
  'HEAD',
  'OPTIONS',
];

// --- operations ---

export const addOperation = (ops: ApiOperation[]): ApiOperation[] => [
  ...ops,
  { method: 'GET', path: '/' },
];

export const updateOperation = (
  ops: ApiOperation[],
  index: number,
  patch: Partial<ApiOperation>
): ApiOperation[] =>
  ops.map((op, i) => (i === index ? { ...op, ...patch } : op));

export const removeOperation = (
  ops: ApiOperation[],
  index: number
): ApiOperation[] => ops.filter((_, i) => i !== index);

// --- policies ---

export const addPolicy = (policies: ApiPolicy[]): ApiPolicy[] => [
  ...policies,
  { name: '', version: '1.0.0' },
];

export const updatePolicy = (
  policies: ApiPolicy[],
  index: number,
  patch: Partial<ApiPolicy>
): ApiPolicy[] =>
  policies.map((p, i) => (i === index ? { ...p, ...patch } : p));

export const removePolicy = (
  policies: ApiPolicy[],
  index: number
): ApiPolicy[] => policies.filter((_, i) => i !== index);

export const replacePolicy = (
  policies: ApiPolicy[],
  index: number,
  policy: ApiPolicy
): ApiPolicy[] => policies.map((p, i) => (i === index ? policy : p));

/** Moves a policy one slot up (-1) or down (+1); order is persisted. */
export const movePolicy = (
  policies: ApiPolicy[],
  index: number,
  direction: -1 | 1
): ApiPolicy[] => {
  const target = index + direction;
  if (target < 0 || target >= policies.length) return policies;
  const next = [...policies];
  [next[index], next[target]] = [next[target], next[index]];
  return next;
};

/** Reorders a policy from one index to another (drag-and-drop reorder). */
export const reorderPolicies = (
  policies: ApiPolicy[],
  from: number,
  to: number
): ApiPolicy[] => {
  if (
    from === to ||
    from < 0 ||
    to < 0 ||
    from >= policies.length ||
    to >= policies.length
  ) {
    return policies;
  }
  const next = [...policies];
  const [moved] = next.splice(from, 1);
  next.splice(to, 0, moved);
  return next;
};

// --- backend-resource mapping (proxy resource → backend path) ---
//
// platform-api has no backend-operation model, so "map /sample → /book" is
// expressed as a path-rewrite POLICY on the operation. These constants must
// match the rewrite policy published by the Policy Hub / understood by the
// gateway.
export const REWRITE_POLICY_NAME = 'mediation.rewrite_resource_path';
export const REWRITE_POLICY_VERSION = '2.0.0';
export const REWRITE_PATH_PARAM = 'New Path';

/** The backend path an operation maps to (from its rewrite policy), if any. */
export const getBackendPath = (op: ApiOperation): string | undefined => {
  const policy = (op.policies || []).find((p) => p.name === REWRITE_POLICY_NAME);
  const value = policy?.params?.[REWRITE_PATH_PARAM];
  return typeof value === 'string' && value ? value : undefined;
};

/**
 * Sets (or clears) an operation's backend path by upserting/removing the
 * rewrite policy, preserving any other policies on the operation.
 */
export const setBackendPath = (
  op: ApiOperation,
  backendPath: string | undefined
): ApiOperation => {
  const others = (op.policies || []).filter((p) => p.name !== REWRITE_POLICY_NAME);
  if (!backendPath) {
    return { ...op, policies: others };
  }
  return {
    ...op,
    policies: [
      ...others,
      {
        name: REWRITE_POLICY_NAME,
        version: REWRITE_POLICY_VERSION,
        params: { [REWRITE_PATH_PARAM]: backendPath },
      },
    ],
  };
};

/** Distinct backend paths currently mapped across all operations. */
export const backendPathsFromOperations = (ops: ApiOperation[]): string[] => {
  const set = new Set<string>();
  for (const op of ops) {
    const path = getBackendPath(op);
    if (path) set.add(path);
  }
  return [...set];
};

// --- method chip colors (legacy: distinct color per HTTP verb) ---

export type ChipColor =
  | 'default'
  | 'primary'
  | 'secondary'
  | 'success'
  | 'error'
  | 'warning'
  | 'info';

export const methodColor = (method: string): ChipColor => {
  switch (method.toUpperCase()) {
    case 'GET':
      return 'success';
    case 'POST':
      return 'primary';
    case 'PUT':
      return 'warning';
    case 'DELETE':
      return 'error';
    case 'PATCH':
      return 'secondary';
    default:
      return 'default';
  }
};

// --- validation ---

/** A URL is acceptable when empty (optional) or a well-formed http(s) URL. */
export const isValidUrl = (value: string): boolean => {
  const v = value.trim();
  if (v === '') return true;
  try {
    const u = new URL(v);
    return u.protocol === 'http:' || u.protocol === 'https:';
  } catch {
    return false;
  }
};

/** Operations are valid when every row has a non-empty, leading-slash path. */
export const operationsValid = (ops: ApiOperation[]): boolean =>
  ops.every((op) => op.path.trim().startsWith('/'));

/** Policies are valid when every row has a name + version. */
export const policiesValid = (policies: ApiPolicy[]): boolean =>
  policies.every((p) => p.name.trim() !== '' && p.version.trim() !== '');

/** Merges edited develop fields back onto a detail for submission. */
export const withRoutingEdits = (
  detail: ApiDetail,
  edits: { operations: ApiOperation[]; prodUrl?: string; sandboxUrl?: string }
): ApiDetail => ({
  ...detail,
  operations: edits.operations,
  endpoints: {
    prodUrl: edits.prodUrl?.trim() || undefined,
    sandboxUrl: edits.sandboxUrl?.trim() || undefined,
  },
});
