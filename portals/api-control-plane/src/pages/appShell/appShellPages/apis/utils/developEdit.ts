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
  Operation,
  Policy,
  RestApi,
  UpdateRestApiBody,
  UpstreamDefinition,
} from '@/api/resources/restApis';

/**
 * Editing model for the Develop section, and the two conversions that connect
 * it to the wire.
 *
 * The spec nests an operation's method, path and policies under `request`,
 * which is right for the wire and wrong for a form: every field the Routing
 * canvas edits would sit one level down from the two (`name`, `description`)
 * that do not. So the panels edit a *flattened* operation and convert at the
 * boundary — `toEditableOperations` on load, `withRoutingEdits` /
 * `withPolicyEdits` on save.
 *
 * The flat type is **derived** from `Operation`, not restated: adding a field
 * to `OperationRequest` in the spec adds it here, and renaming one breaks this
 * file at compile time.
 */
export type EditableOperation = Pick<Operation, 'name' | 'description'> & Operation['request'];

export type HttpMethod = Operation['request']['method'];

export const HTTP_METHODS: HttpMethod[] = [
  'GET',
  'POST',
  'PUT',
  'DELETE',
  'PATCH',
  'HEAD',
  'OPTIONS',
];

// --- wire ↔ editing model ---

/** Flattens a fetched API's operations into the panels' editing model. */
export const toEditableOperations = (api: RestApi): EditableOperation[] =>
  (api.operations ?? []).map((operation) => ({
    name: operation.name,
    description: operation.description,
    ...operation.request,
  }));

/** Re-nests the editing model into the spec's `Operation` shape. */
export const toSpecOperations = (operations: EditableOperation[]): Operation[] =>
  operations.map(({ name, description, ...request }) => ({
    name,
    description,
    request,
  }));

// --- operations ---

export const addOperation = (ops: EditableOperation[]): EditableOperation[] => [
  ...ops,
  { method: 'GET', path: '/' },
];

export const updateOperation = (
  ops: EditableOperation[],
  index: number,
  patch: Partial<EditableOperation>,
): EditableOperation[] => ops.map((op, i) => (i === index ? { ...op, ...patch } : op));

export const removeOperation = (ops: EditableOperation[], index: number): EditableOperation[] =>
  ops.filter((_, i) => i !== index);

// --- policies ---

export const addPolicy = (policies: Policy[]): Policy[] => [
  ...policies,
  { name: '', version: '1.0.0' },
];

export const updatePolicy = (policies: Policy[], index: number, patch: Partial<Policy>): Policy[] =>
  policies.map((p, i) => (i === index ? { ...p, ...patch } : p));

export const removePolicy = (policies: Policy[], index: number): Policy[] =>
  policies.filter((_, i) => i !== index);

export const replacePolicy = (policies: Policy[], index: number, policy: Policy): Policy[] =>
  policies.map((p, i) => (i === index ? policy : p));

/** Moves a policy one slot up (-1) or down (+1); order is persisted. */
export const movePolicy = (policies: Policy[], index: number, direction: -1 | 1): Policy[] => {
  const target = index + direction;
  if (target < 0 || target >= policies.length) return policies;
  const next = [...policies];
  [next[index], next[target]] = [next[target], next[index]];
  return next;
};

/** Reorders a policy from one index to another (drag-and-drop reorder). */
export const reorderPolicies = (policies: Policy[], from: number, to: number): Policy[] => {
  if (from === to || from < 0 || to < 0 || from >= policies.length || to >= policies.length) {
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
export const getBackendPath = (op: EditableOperation): string | undefined => {
  const policy = (op.policies || []).find((p) => p.name === REWRITE_POLICY_NAME);
  const value = policy?.params?.[REWRITE_PATH_PARAM];
  return typeof value === 'string' && value ? value : undefined;
};

/**
 * Sets (or clears) an operation's backend path by upserting/removing the
 * rewrite policy, preserving any other policies on the operation.
 */
export const setBackendPath = (
  op: EditableOperation,
  backendPath: string | undefined,
): EditableOperation => {
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
export const backendPathsFromOperations = (ops: EditableOperation[]): string[] => {
  const set = new Set<string>();
  for (const op of ops) {
    const path = getBackendPath(op);
    if (path) set.add(path);
  }
  return [...set];
};

// --- method chip colors (legacy: distinct color per HTTP verb) ---

export type ChipColor =
  'default' | 'primary' | 'secondary' | 'success' | 'error' | 'warning' | 'info';

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
export const operationsValid = (ops: EditableOperation[]): boolean =>
  ops.every((op) => op.path.trim().startsWith('/'));

/** Policies are valid when every row has a name + version. */
export const policiesValid = (policies: Policy[]): boolean =>
  policies.every((p) => p.name.trim() !== '' && p.version.trim() !== '');

// --- update bodies ---

/**
 * Applies an edited URL to one side of `upstream`.
 *
 * Everything else on that side is carried through — most importantly `auth`,
 * which holds the upstream's credentials. The previous adapter rebuilt
 * `upstream` as `{ main: { url } }`, so saving the Routing panel silently
 * discarded the backend's authentication config.
 *
 * `url` and `ref` are mutually exclusive in the spec, so setting one clears the
 * other; clearing the URL entirely leaves any `ref` in place.
 */
const withUpstreamUrl = (
  existing: UpstreamDefinition | undefined,
  url: string | undefined,
): UpstreamDefinition => {
  const next: UpstreamDefinition = { ...(existing ?? {}) };
  const trimmed = url?.trim();
  if (trimmed) {
    next.url = trimmed;
    delete next.ref;
  } else {
    delete next.url;
  }
  return next;
};

/**
 * The PUT body for a Routing save.
 *
 * The spec's update body is the whole `RESTAPI`, so the fetched object is
 * spread back with only the edited fields replaced — that is what preserves
 * server-managed fields without a separate `raw` copy to merge against.
 */
export const withRoutingEdits = (
  api: RestApi,
  edits: { operations: EditableOperation[]; prodUrl?: string; sandboxUrl?: string },
): UpdateRestApiBody => {
  const sandboxUrl = edits.sandboxUrl?.trim();
  return {
    ...api,
    operations: toSpecOperations(edits.operations),
    upstream: {
      ...api.upstream,
      main: withUpstreamUrl(api.upstream?.main, edits.prodUrl),
      // Clearing the sandbox URL removes the sandbox side outright, rather
      // than leaving a definition with neither `url` nor `ref`.
      ...(sandboxUrl
        ? { sandbox: withUpstreamUrl(api.upstream?.sandbox, sandboxUrl) }
        : { sandbox: undefined }),
    },
  };
};

/** The PUT body for a Policies save — API-level policies plus per-operation ones. */
export const withPolicyEdits = (
  api: RestApi,
  edits: { policies: Policy[]; operations: EditableOperation[] },
): UpdateRestApiBody => ({
  ...api,
  policies: edits.policies,
  operations: toSpecOperations(edits.operations),
});
