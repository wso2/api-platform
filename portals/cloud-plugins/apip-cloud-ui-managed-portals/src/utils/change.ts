/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import type { ManagedPortal, ManagedPortalChange } from '../types';

/** What a running background change is doing, for a pending portal's status. */
export function describeChange(change?: ManagedPortalChange): string {
  switch (change?.op) {
    case 'loginEnvironment':
      return change.loginEnvironment ? `Moving sign-in to ${change.loginEnvironment}...` : 'Moving sign-in...';
    case 'selfSignup':
      return change.selfSignup ? 'Turning sign-up on...' : 'Turning sign-up off...';
    case 'rotateKey':
      return 'Rotating the portal key...';
    default:
      return 'Updating portal...';
  }
}

/** A pending portal that existed before: its change, not its creation, is running. */
export function isBeingChanged(portal: ManagedPortal): boolean {
  return portal.status === 'pending' && Boolean(portal.change);
}

/** A failed portal whose last change failed, rather than its creation. */
export function changeFailed(portal: ManagedPortal): boolean {
  return portal.status === 'failed' && Boolean(portal.lastChangeError);
}
