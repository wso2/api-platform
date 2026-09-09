/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 *
 * Mirrors `CloudHostPort` from api-platform's `portals/api-control-plane/src/hostPort.tsx`.
 * Duplicated, not imported — api-platform and apim-saas are separate git
 * repos. Kept deliberately small and stable; keep this in sync by hand if
 * the upstream shape changes. This feature package only ever receives a
 * value of this shape as a plain prop (`render(port)` / `<ManagedPortalsPage
 * port={port} />`) — never a shared context object, so there's no
 * cross-repo context-identity problem to worry about.
 */

export type NotifySeverity = 'success' | 'info' | 'warning' | 'error';

export type CloudHostPort = {
  orgHandle: string;
  projectHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
};
