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
 * The API layer's outbound events — the two things the server can say that the
 * rest of the app has to react to: "this request had no live session" (401) and
 * "this session may not do that" (403).
 *
 * This lives in its own module, apart from the transport, because the two have
 * opposite audiences. The transport is internal — nothing outside `src/api`
 * should issue a request. This event bus is the layer's *public* surface: the
 * auth provider is expected to subscribe, and the layer would be incomplete
 * without something doing so.
 *
 * Publishing rather than calling into `AuthProvider` directly keeps the
 * dependency one-way. The transport importing the auth provider would be a
 * cycle, and would make the HTTP client impossible to test without mounting
 * React. The 403 channel exists for the same reason: the transport cannot ask
 * `PermissionProvider` what it had predicted without importing React.
 */

/* -------------------------------------------------------------------------- */
/* Unauthorized (401)                                                          */
/* -------------------------------------------------------------------------- */

/** Re-checks the session and resolves whether it is still alive. */
export type UnauthorizedHandler = () => Promise<boolean>;

let unauthorizedHandler: UnauthorizedHandler | undefined;

/** The check in flight, shared by every 401 that arrives while it runs. */
let pending: Promise<boolean> | undefined;

/**
 * Registers what a 401 triggers. Returns an unregister function suitable for
 * returning straight from a `useEffect`.
 */
export const onUnauthorized = (handler: UnauthorizedHandler): (() => void) => {
  unauthorizedHandler = handler;
  return () => {
    if (unauthorizedHandler === handler) unauthorizedHandler = undefined;
  };
};

/**
 * Announces a 401 and resolves whether the session is still alive, so the
 * request is worth sending again. Parallel 401s share one check: a dashboard
 * with eight failing queries asks once. False when nothing is registered or the
 * check fails.
 */
export const notifyUnauthorized = (): Promise<boolean> => {
  const handler = unauthorizedHandler;
  if (!handler) return Promise.resolve(false);
  pending ??= Promise.resolve()
    .then(handler)
    .catch(() => false)
    .finally(() => {
      pending = undefined;
    });
  return pending;
};

/* -------------------------------------------------------------------------- */
/* Forbidden (403)                                                             */
/* -------------------------------------------------------------------------- */

/**
 * A 403 the server returned, tagged with the operation that drew it.
 *
 * The operation is what makes this worth publishing rather than handling in
 * place: it is the key `PermissionProvider` needs to ask whether the console had
 * predicted the call would be *allowed*; a mismatch between the generated scope
 * map and the deployed backend, which is otherwise invisible.
 */
export type ForbiddenEvent = {
  /** The spec `operationId`, when the caller supplied one. */
  operation?: string;
};

type ForbiddenListener = (event: ForbiddenEvent) => void;

const forbiddenListeners = new Set<ForbiddenListener>();

/**
 * Subscribes to authorization failures. Returns an unsubscribe function
 * suitable for returning straight from a `useEffect`.
 */
export const onForbidden = (listener: ForbiddenListener): (() => void) => {
  forbiddenListeners.add(listener);
  return () => {
    forbiddenListeners.delete(listener);
  };
};

/**
 * Debounce repeated failures per operation, rather than globally. Different
 * operations represent distinct authorization events; only repeated failures
 * for the same operation are suppressed.
 */
const FORBIDDEN_DEBOUNCE_MS = 3_000;

const lastForbiddenAt = new Map<string, number>();

/** Announces that the server refused an operation for this session. */
export const notifyForbidden = (operation?: string): void => {
  const key = operation ?? '';
  const now = Date.now();
  const previous = lastForbiddenAt.get(key) ?? 0;
  if (now - previous < FORBIDDEN_DEBOUNCE_MS) return;
  lastForbiddenAt.set(key, now);
  for (const listener of forbiddenListeners) listener({ operation });
};

/** Clears the per-operation debounce windows. Test seam, as above. */
export const resetForbiddenNotice = (): void => {
  lastForbiddenAt.clear();
};
