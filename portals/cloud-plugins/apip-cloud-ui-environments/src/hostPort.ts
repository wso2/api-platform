/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 *
 * Mirrors `CloudHostPort` from api-platform's `portals/api-control-plane/src/hostPort.tsx`.
 * Duplicated, not imported, even though both now live in this same repo: this
 * package only ever receives a value of this shape as a plain prop
 * (`render(port)` / `<EnvironmentsPage port={port} />`) — never a shared
 * context object — so there's no cross-portal context-identity problem to
 * worry about, and the package stays reusable by any future host with the
 * same small port shape without an internal cross-portal import. Kept
 * deliberately small and stable; keep this in sync by hand if the upstream
 * shape changes.
 */

export type NotifySeverity = 'success' | 'info' | 'warning' | 'error';

export type CloudHostPort = {
  orgHandle: string;
  projectHandle?: string;
  navigate: (path: string) => void;
  notify: (message: string, severity?: NotifySeverity) => void;
};
