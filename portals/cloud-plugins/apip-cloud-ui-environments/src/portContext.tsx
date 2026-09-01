/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 *
 * This feature's own data-injection seam — local to this package only, never
 * crossing into a host portal's own context (the boundary-crossing shape is
 * `CloudHostPort`, received as a plain prop by `EnvironmentsPage`). Kept as a
 * context purely so `useEnvironmentList` doesn't need `EnvironmentPort`/
 * `notify` prop-drilled through every list/dialog component.
 */
import { createContext, useContext, type ReactNode } from 'react';

import type { CloudHostPort } from './hostPort';
import type { EnvironmentPort } from './types';

type EnvironmentFeatureContextValue = {
  port: EnvironmentPort;
  host: CloudHostPort;
};

const EnvironmentFeatureContext = createContext<EnvironmentFeatureContextValue | null>(null);

export function EnvironmentFeatureProvider({
  value,
  children,
}: {
  value: EnvironmentFeatureContextValue;
  children: ReactNode;
}) {
  return (
    <EnvironmentFeatureContext.Provider value={value}>{children}</EnvironmentFeatureContext.Provider>
  );
}

export function useEnvironmentFeature(): EnvironmentFeatureContextValue {
  const ctx = useContext(EnvironmentFeatureContext);
  if (!ctx) {
    throw new Error(
      'useEnvironmentFeature must be used within an EnvironmentFeatureProvider (rendered by EnvironmentsPage)'
    );
  }
  return ctx;
}
