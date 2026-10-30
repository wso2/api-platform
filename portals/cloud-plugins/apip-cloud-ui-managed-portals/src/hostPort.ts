/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

// Duplicated (not imported) from api-platform's api-control-plane/hostPort so the two repos stay decoupled; keep in sync by hand.

export type NotifySeverity = 'success' | 'info' | 'warning' | 'error';

export type CloudHostPort = {
  orgHandle: string;
  projectHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
  /**
   * Quota, when the mounting console tracks any. `refresh` after a create or
   * delete; `canCreate`/`limitMessage` gate this feature's own create button.
   * Optional: a host without entitlements omits it and nothing is capped.
   */
  /**
   * Tells the host that data it also caches has changed. This feature writes
   * through its own client, which the host's cache cannot see, so without it
   * the host's counts stay stale. Optional: a host that caches nothing omits it.
   */
  resources?: {
    invalidate: (resource: 'apiPortals') => void;
  };

  resourceLimits?: {
    refresh: () => void;
    canCreate?: (component: 'portals') => boolean;
    limitMessage?: (component: 'portals') => string;
    /**
     * The organization may not write at all — a different question from
     * whether it has room for one more portal, and the one that governs
     * editing an existing one. `canCreate` already answers `false` while this
     * holds, so the create button needs nothing extra; an edit has no count to
     * consult and so must read this directly.
     *
     * Optional, and `false` when absent: a host with no entitlement supplier
     * omits it and every control stays enabled exactly as before.
     */
    readOnly?: boolean;
    /** Why the organization is read-only; the host supplies the wording. */
    readOnlyMessage?: string;
  };
};
