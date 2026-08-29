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
 * The API layer's outbound event: "the server says this session is gone".
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
 * React.
 */

type SessionExpiredListener = () => void;

const listeners = new Set<SessionExpiredListener>();

/**
 * Subscribes to session expiry. Returns an unsubscribe function suitable for
 * returning straight from a `useEffect`.
 */
export const onSessionExpired = (listener: SessionExpiredListener): (() => void) => {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
};

/** Window within which repeat notifications collapse into one. */
const DEBOUNCE_MS = 3_000;

let lastNotifiedAt = 0;

/**
 * Announces that the session is gone.
 *
 * Debounced because a dashboard commonly has several queries in flight at once:
 * without it, eight parallel 401s would queue eight re-checks and eight
 * redirects to the login page.
 */
export const notifySessionExpired = (): void => {
  const now = Date.now();
  if (now - lastNotifiedAt < DEBOUNCE_MS) return;
  lastNotifiedAt = now;
  for (const listener of listeners) listener();
};

/**
 * Clears the debounce window so the next notification fires again.
 *
 * Test seam, like `resetHttpClient`: the window is module state, so without
 * this a spec that triggers a 401 would silently suppress the notification in
 * every spec running within the next few seconds, making tests order-dependent.
 */
export const resetSessionExpiryNotice = (): void => {
  lastNotifiedAt = 0;
};
