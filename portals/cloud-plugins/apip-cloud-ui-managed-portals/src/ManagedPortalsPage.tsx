/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useMemo, useState } from 'react';
import { PageContent, Typography } from '@wso2/oxygen-ui';

import type { CloudHostPort } from './hostPort';
import ManagedPortalDetail from './ManagedPortalDetail';
import ManagedPortalsList from './ManagedPortalsList';
import { PortalFeatureProvider } from './portContext';
import { createRealPortalPort, resolveApiBase } from './realPort';

export type ManagedPortalsPageProps = {
  /** Host capabilities supplied by the mounting console; kept as a prop so the feature stays host-agnostic. */
  port: CloudHostPort;
};

export function ManagedPortalsPage({ port }: ManagedPortalsPageProps) {
  // Fail closed when the platform-api base is missing; tests / storybook build the mock port directly.
  const portalPort = useMemo(() => {
    const base = resolveApiBase();
    return base ? createRealPortalPort(base, port.orgHandle) : null;
  }, [port.orgHandle]);

  // Local state (no URL param) keeps react-router out of this feature package; refresh loses the selection.
  const [selectedId, setSelectedId] = useState<string | null>(null);

  if (!portalPort) {
    return (
      <PageContent fullWidth>
        <Typography variant="h5">Managed API Portals</Typography>
        <Typography color="error" sx={{ mt: 2 }}>
          Managed API Portals is not available: platform-api base URL is not
          configured. Set window.__RUNTIME_CONFIG__.platformApiBaseUrl (or
          window.config.platformApiBaseUrl) on the host to enable this feature.
        </Typography>
      </PageContent>
    );
  }

  return (
    <PortalFeatureProvider value={{ port: portalPort, host: port }}>
      {selectedId ? (
        <ManagedPortalDetail id={selectedId} onBack={() => setSelectedId(null)} />
      ) : (
        <ManagedPortalsList onSelect={setSelectedId} />
      )}
    </PortalFeatureProvider>
  );
}
