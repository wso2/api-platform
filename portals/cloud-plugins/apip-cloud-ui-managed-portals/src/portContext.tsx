/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

// Package-local context so hooks aren't forced to prop-drill port/notify through every dialog.
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
