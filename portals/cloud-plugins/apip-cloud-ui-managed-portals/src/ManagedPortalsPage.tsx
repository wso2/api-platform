/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useEffect, useMemo, useState } from 'react';
import { PageContent, Typography } from '@wso2/oxygen-ui';

import type { CloudHostPort } from './hostPort';
import ManagedPortalCreate from './ManagedPortalCreate';
import ManagedPortalEdit from './ManagedPortalEdit';
import ManagedPortalsList from './ManagedPortalsList';
import { PortalFeatureProvider } from './portContext';
import { createRealPortalPort, resolveApiBase } from './realPort';

export type ManagedPortalsPageProps = {
  /** Host capabilities supplied by the mounting console; kept as a prop so the feature stays host-agnostic. */
  port: CloudHostPort;
};

/**
 * Sub-view state. Kept as a discriminated union so `portalId` cannot be set
 * without an edit view, and vice versa - no react-router in this feature package.
 *
 * Edit carries an id, not the record from the list: the list projection strips
 * loginEnvironment, so the edit view has to GET the full portal to seed its form.
 */
type PageView =
  | { kind: 'list' }
  | { kind: 'create' }
  | { kind: 'edit'; portalId: string };

export function ManagedPortalsPage({ port }: ManagedPortalsPageProps) {
  // Fail closed when the platform-api base is missing; tests / storybook build the mock port directly.
  const portalPort = useMemo(() => {
    const base = resolveApiBase();
    return base ? createRealPortalPort(base, port.orgHandle) : null;
  }, [port.orgHandle]);

  const [view, setView] = useState<PageView>({ kind: 'list' });

  // Reset to the list when the org switches: an edit view of the previous
  // org's portalId is meaningless in the new org, and a create view carries
  // form state that shouldn't survive the switch (env list changes shape).
  useEffect(() => {
    setView({ kind: 'list' });
  }, [port.orgHandle]);

  if (!portalPort) {
    return (
      <PageContent fullWidth>
        <Typography variant="h5">Portals</Typography>
        <Typography color="error" sx={{ mt: 2 }}>
          Portals is not available: platform-api base URL is not
          configured. Set window.__RUNTIME_CONFIG__.platformApiBaseUrl (or
          window.config.platformApiBaseUrl) on the host to enable this feature.
        </Typography>
      </PageContent>
    );
  }

  const goToList = () => setView({ kind: 'list' });

  // Keying the child views on orgHandle forces a remount when the user
  // switches org while the create/edit view is active. Without this, the
  // form's local state (loginEnvironment picked from the previous org's env
  // list, in-progress name/description) survives the switch and can be
  // submitted against the new org's environment set.
  return (
    <PortalFeatureProvider value={{ port: portalPort, host: port }}>
      {view.kind === 'create' ? (
        <ManagedPortalCreate key={port.orgHandle} onCancel={goToList} onCreated={goToList} />
      ) : view.kind === 'edit' ? (
        <ManagedPortalEdit key={port.orgHandle} portalId={view.portalId} onCancel={goToList} onSaved={goToList} />
      ) : (
        <ManagedPortalsList
          onCreate={() => setView({ kind: 'create' })}
          onEdit={(portal) => setView({ kind: 'edit', portalId: portal.id })}
        />
      )}
    </PortalFeatureProvider>
  );
}
