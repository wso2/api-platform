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

/** The host's same-origin platform-api call; `path` is relative to the API base (e.g. `/environments`). */
export type ApiFetch = <T = unknown>(
  method: string,
  path: string,
  body?: unknown
) => Promise<T | undefined>;

export type CloudHostPort = {
  orgHandle: string;
  projectHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
};
