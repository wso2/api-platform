/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

export { ManagedPortalsPage, type ManagedPortalsPageProps } from './ManagedPortalsPage';
export type { CloudHostPort, NotifySeverity } from './hostPort';
export { createMockPortalPort } from './mockPort';
export { createRealPortalPort, resolveApiBase } from './realPort';
export { useManagedPortalList } from './hooks';
export type {
  CreateManagedPortalInput,
  ManagedPortal,
  PortalPort,
  UpdateManagedPortalInput,
} from './types';
