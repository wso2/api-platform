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

export const HANDLE_MIN_LENGTH = 3;
export const HANDLE_MAX_LENGTH = 40;

/** The server refuses this id: `/service-accounts/token` is the token endpoint. */
const RESERVED_HANDLES = new Set(['token']);

export type HandleProblem = 'length' | 'format' | 'reserved';

/** Display name → a handle the server will accept, or '' when none can be made. */
export const toHandle = (displayName: string): string =>
  displayName
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .slice(0, HANDLE_MAX_LENGTH)
    .replace(/^-+|-+$/g, '');

/**
 * Mirrors platform-api's `ValidateHandle` plus the spec's 40-character cap, so
 * most mistakes show before submit. The server still has the last word.
 */
export const handleProblem = (handle: string): HandleProblem | null => {
  if (handle.length < HANDLE_MIN_LENGTH || handle.length > HANDLE_MAX_LENGTH) return 'length';
  if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(handle)) return 'format';
  if (RESERVED_HANDLES.has(handle)) return 'reserved';
  return null;
};
