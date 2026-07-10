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

import { CSRF_HEADER, CSRF_VALUE } from '../config.env';

export const BASIC_AUTH_SESSION_KEY = 'basic_auth_session';

// Retained as a no-op shim: in BFF mode the session lives in the HttpOnly cookie,
// not localStorage. logout.ts still imports this for its cleanup sweep.
export function clearBasicAuthSession() {
  localStorage.removeItem(BASIC_AUTH_SESSION_KEY);
}

/**
 * POST /api/login — the BFF validates the credentials against the Platform API,
 * stores the issued JWT server-side, and sets the HttpOnly session cookie. No
 * token ever reaches the browser.
 */
export async function basicAuthLogin(username: string, password: string): Promise<void> {
  const res = await fetch('/api/login', {
    method: 'POST',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
      [CSRF_HEADER]: CSRF_VALUE,
    },
    body: JSON.stringify({ username, password }),
  });

  if (!res.ok) {
    const data = await res.json().catch(() => ({})) as Record<string, unknown>;
    throw new Error((data.error as string | undefined) ?? 'Login failed');
  }
}
