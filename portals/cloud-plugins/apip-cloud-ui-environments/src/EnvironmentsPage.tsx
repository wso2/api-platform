/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useMemo } from 'react';

import EnvironmentsList from './EnvironmentsList';
import type { CloudHostPort } from './hostPort';
import { createMockEnvironmentPort } from './mockPort';
import { EnvironmentFeatureProvider } from './portContext';

export type EnvironmentsPageProps = {
  /**
   * Host capabilities, supplied by whichever console mounts this feature —
   * never imported directly. This is what makes the component reusable
   * across more than one host app.
   */
  port: CloudHostPort;
};

export function EnvironmentsPage({ port }: EnvironmentsPageProps) {
  // No real backend exists yet, so each mount gets its own in-memory port.
  // A real, BFF-backed EnvironmentPort drops in here later with no change to
  // EnvironmentsList/useEnvironmentList, which only ever see the interface.
  const environmentPort = useMemo(() => createMockEnvironmentPort(), []);

  return (
    <EnvironmentFeatureProvider value={{ port: environmentPort, host: port }}>
      <EnvironmentsList />
    </EnvironmentFeatureProvider>
  );
}
