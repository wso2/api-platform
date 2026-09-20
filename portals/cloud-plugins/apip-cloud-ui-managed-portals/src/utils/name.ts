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
 * The portal handle limit that fits inside OpenChoreo's RenderedRelease-name
 * label without overflowing the 63-char K8s label ceiling. Backend accepts up
 * to 40, but the derived label with its fixed prefix + hash suffix caps the
 * practical limit lower — keep the UI aligned with what actually succeeds so
 * users don't have to submit to discover the ceiling.
 */
export const MAX_PORTAL_HANDLE_LENGTH = 34;

/** Minimum length the UI enforces for readability; backend has no explicit floor. */
export const MIN_PORTAL_HANDLE_LENGTH = 3;

/**
 * Derives the handle a portal will be addressed by from its display name, the
 * same conversion the backend applies: lowercased, with spaces and underscores
 * folded to hyphens and anything else dropped.
 *
 * Returns `''` when nothing usable survives, which the caller reports rather
 * than substituting a name of its own.
 */
export function portalHandleFromName(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[\s_]+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .replace(/-{2,}/g, '-')
    .replace(/^-+|-+$/g, '');
}

/**
 * Validates a new portal's name against the same rules the backend applies,
 * returning the message to show or `undefined` when the name is fine.
 *
 * The name is a display name: it may be written however the user likes, and
 * the handle is derived from it, so casing and spaces are not errors. What it
 * cannot be is a name no handle can be built from, or one whose derived handle
 * does not fit the label ceiling. An empty name is left to the field's own
 * `required` handling rather than reported here.
 */
export function validatePortalName(name: string): string | undefined {
  if (!name.trim()) return undefined;

  const handle = portalHandleFromName(name);
  if (!handle) {
    return 'Include at least one letter or number.';
  }
  if (handle.length < MIN_PORTAL_HANDLE_LENGTH) {
    return `The derived handle "${handle}" is too short; use at least ${MIN_PORTAL_HANDLE_LENGTH} letters or digits.`;
  }
  if (handle.length > MAX_PORTAL_HANDLE_LENGTH) {
    return `The derived handle "${handle}" exceeds ${MAX_PORTAL_HANDLE_LENGTH} characters; shorten the name.`;
  }
  return undefined;
}
