/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
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
    return `The derived handle "${handle}" is too short; use at least ${MIN_PORTAL_HANDLE_LENGTH} characters.`;
  }
  if (handle.length > MAX_PORTAL_HANDLE_LENGTH) {
    return `The derived handle "${handle}" exceeds ${MAX_PORTAL_HANDLE_LENGTH} characters; shorten the name.`;
  }
  return undefined;
}
