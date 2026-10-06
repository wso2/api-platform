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

// Mirrors the server's handle rules, so a bad ID is caught before submit.
export const HANDLE_MIN_LENGTH = 3;
export const HANDLE_MAX_LENGTH = 40;

const RESERVED_HANDLES = new Set(['token']);

export type HandleProblem = 'length' | 'format' | 'reserved';

export const toHandle = (displayName: string): string =>
  displayName
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .slice(0, HANDLE_MAX_LENGTH)
    .replace(/^-+|-+$/g, '');

export const handleProblem = (handle: string): HandleProblem | null => {
  if (handle.length < HANDLE_MIN_LENGTH || handle.length > HANDLE_MAX_LENGTH) return 'length';
  if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(handle)) return 'format';
  if (RESERVED_HANDLES.has(handle)) return 'reserved';
  return null;
};

// The browser only knows the BFF's proxy, which a workload cannot use, so the
// examples name the real address as a shell variable.
const API_BASE = '$PLATFORM_API_URL/api/v0.9';

export const tokenRequestCurl = (clientId: string, scopes: readonly string[]): string =>
  [
    `curl -X POST "${API_BASE}/service-accounts/token" \\`,
    '  -d grant_type=client_credentials \\',
    `  -d client_id=${clientId} \\`,
    scopes.length > 0 ? '  -d client_secret="$CLIENT_SECRET" \\' : '  -d client_secret="$CLIENT_SECRET"',
    ...(scopes.length > 0 ? [`  -d scope="${scopes.join(' ')}"`] : []),
  ].join('\n');

export const tokenUseCurl = (): string =>
  ['curl -H "Authorization: Bearer $ACCESS_TOKEN" \\', `  "${API_BASE}/projects"`].join('\n');

/** Two letters from the display name: the first two words, or one word's first two letters. */
export const initials = (name: string): string => {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
};

