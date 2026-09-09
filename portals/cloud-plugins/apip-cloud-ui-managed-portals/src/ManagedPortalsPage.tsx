/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useMemo, useState } from 'react';

import type { CloudHostPort } from './hostPort';
import ManagedPortalDetail from './ManagedPortalDetail';
import ManagedPortalsList from './ManagedPortalsList';
import { createMockPortalPort } from './mockPort';
import { PortalFeatureProvider } from './portContext';
import { createRealPortalPort, resolveApiBase } from './realPort';

export type ManagedPortalsPageProps = {
  /**
   * Host capabilities, supplied by whichever console mounts this feature —
   * never imported directly. This is what makes the component reusable across
   * more than one host app.
   */
  port: CloudHostPort;
};

export function ManagedPortalsPage({ port }: ManagedPortalsPageProps) {
  // Construct the port once per mount: the real, BFF-backed port when the
  // console exposes a platform-api base, otherwise an in-memory mock (tests /
  // storybook). ManagedPortalsList/useManagedPortalList only ever see PortalPort.
  const portalPort = useMemo(() => {
    const base = resolveApiBase();
    return base ? createRealPortalPort(base, port.orgHandle) : createMockPortalPort();
  }, [port.orgHandle]);

  // Which portal, if any, is being viewed in detail. Kept as local state (not
  // a URL param) to avoid dragging react-router into the feature package —
  // the same choice apip-cloud-ui-gateways makes. A refresh loses the current
  // selection, which is acceptable for a first cut; revisit if the console
  // grows deep-link requirements.
  const [selectedId, setSelectedId] = useState<string | null>(null);

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
