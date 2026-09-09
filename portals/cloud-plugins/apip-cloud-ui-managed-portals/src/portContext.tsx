/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 *
 * This feature's own data-injection seam — local to this package only, never
 * crossing the api-platform/apim-saas boundary (the boundary-crossing shape
 * is `CloudHostPort`, received as a plain prop by `ManagedPortalsPage`).
 * Kept as a context purely so `useManagedPortalList` doesn't need
 * `PortalPort`/`notify` prop-drilled through every list/dialog component.
 */
import { createContext, useContext, type ReactNode } from 'react';

import type { CloudHostPort } from './hostPort';
import type { PortalPort } from './types';

type PortalFeatureContextValue = {
  port: PortalPort;
  host: CloudHostPort;
};

const PortalFeatureContext = createContext<PortalFeatureContextValue | null>(null);

export function PortalFeatureProvider({
  value,
  children,
}: {
  value: PortalFeatureContextValue;
  children: ReactNode;
}) {
  return (
    <PortalFeatureContext.Provider value={value}>{children}</PortalFeatureContext.Provider>
  );
}

export function usePortalFeature(): PortalFeatureContextValue {
  const ctx = useContext(PortalFeatureContext);
  if (!ctx) {
    throw new Error(
      'usePortalFeature must be used within a PortalFeatureProvider (rendered by ManagedPortalsPage)'
    );
  }
  return ctx;
}
