/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { AppShell, Box, Footer, NotificationPanel } from '@wso2/oxygen-ui';
import { Bell } from '@wso2/oxygen-ui-icons-react';
import { Fragment, Suspense, useMemo } from 'react';
import { useQueryClient } from '@tanstack/react-query';

import { useApiScope } from '../../api/core/scope';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

import { getBillingOrganization } from '../../billing/organization';
import { AppPage } from '../../components/AppPage';
import ReadOnlyBanner from '../../components/common/ReadOnlyBanner';
import { ErrorBoundary } from '../../components/errors/ErrorBoundary';
import { PageErrorFallback, SidebarErrorFallback } from '../../components/errors/ErrorFallback';
import { LoadingState } from '../../components/StateViews';
import { runtimeConfig } from '../../config/runtime';
import { useDocumentTitle } from '../../hooks/useDocumentTitle';
import { useResourceLimits } from '../../hooks/useResourceLimits';
import { usePageTitle } from '../../navigation/usePageTitle';
import { useConsoleScope } from '../../scope/ConsoleScopeProvider';
import { useNotifications } from '../../components/Notifications';
import { APP_GATE_SLOT, type ApiControlPlaneAppGate } from '../../extensions';
import { gatewayKeys } from '../../api/resources/gateways/gateways.queries';
import { apiPortalKeys } from '../../api/resources/apiPortals/apiPortals.queries';
import {
  extensionApiFetch,
  PortProvider,
  type CloudHostPort,
  type InvalidatableResource,
} from '../../hostPort';
import { useSlot } from '../../slots';
import { AppHeader } from './AppHeader';
import { APP_FOOTER_ID } from './appLayoutConstants';
import { AppSidebar } from './AppSidebar';
import { FormattedMessage } from 'react-intl';

export default function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { params } = useConsoleScope();
  const { notify } = useNotifications();

  // Every page inside the shell gets its tab title from here, so a new route
  // is named by its sidebar entry without touching the page itself.
  useDocumentTitle(usePageTitle());

  // Read rather than owned here: ResourceLimitsProvider sits above this
  // component (App.tsx) so the value can ride the Port. A cloud extension
  // mounted on `app.gate` supplies the numbers through `resourceLimits.set`;
  // this portal only carries them, and with no such extension every field
  // stays at its inert default.
  const {
    canCreate,
    limitMessage,
    setResourceLimits,
    refresh,
    refreshSignal,
    readOnly,
    readOnlyMessage,
    readOnlyBanner,
    setReadOnly,
  } = useResourceLimits();

  const resourceLimits = useMemo(
    () => ({
      canCreate,
      limitMessage,
      set: setResourceLimits,
      refresh,
      refreshSignal,
      readOnly,
      readOnlyMessage,
      readOnlyBanner,
      setReadOnly,
    }),
    [
      canCreate,
      limitMessage,
      readOnly,
      readOnlyBanner,
      readOnlyMessage,
      refresh,
      refreshSignal,
      setReadOnly,
      setResourceLimits,
    ]
  );

  // Headless policy entries. They render nothing; mounting them is what runs
  // them, and the one that matters today is the entitlement supplier that
  // fills `resourceLimits`. Empty in any deployment registering no extension.
  const appGates = useSlot<ApiControlPlaneAppGate>(APP_GATE_SLOT);

  // An extension writing through its own client cannot reach this cache, so it
  // names what changed and the host maps that to keys. `all(org)`, not the
  // list: a create or delete moves counts and pagination elsewhere too.
  const queryClient = useQueryClient();
  // The org *id* the queries are keyed on, not the URL handle — keying on the
  // handle would invalidate a key nothing uses.
  const { org } = useApiScope();
  const resources = useMemo(
    () => ({
      invalidate: (resource: InvalidatableResource) => {
        if (!org) return;
        const keys = resource === 'gateways' ? gatewayKeys : apiPortalKeys;
        void queryClient.invalidateQueries({ queryKey: keys.all(org) });
      },
    }),
    [org, queryClient]
  );

  // Built once per render from this portal's own hooks, then handed down as
  // a plain value to every extension's `render(port)` — see `hostPort.tsx`
  // for why this crosses the api-platform/apim-saas seam as a value, not a
  // shared context object.
  const port: CloudHostPort = {
    orgHandle: params.orgHandle ?? '',
    projectHandle: params.projectHandler,
    apiHandle: params.apiHandler,
    navigate,
    notify,
    apiFetch: extensionApiFetch,
    billing: { organization: getBillingOrganization },
    gatewayVersions: runtimeConfig.gatewayVersions,
    // Only an explicitly configured host: the console's own fallback can be `localhost`, which
    // is no address for a gateway running somewhere else to dial.
    gatewayControlPlaneHost: runtimeConfig.gatewayControlPlaneHostOverride || undefined,
    resourceLimits,
    resources,
  };

  return (
    <PortProvider value={port}>
      <AppShell initialCollapsed={false} collapseOnSelectOnMobile>
        <AppShell.Navbar>
          <AppHeader />
        </AppShell.Navbar>

        <AppShell.Sidebar>
          {/* Keep this outside <Sidebar> so Sidebar.Category can inspect children */}
          <ErrorBoundary fallback={() => <SidebarErrorFallback />} resetKeys={[location.pathname]}>
            <AppSidebar />
          </ErrorBoundary>
        </AppShell.Sidebar>

        <AppShell.Main>
          {/* `AppShell.Main` is a fragment, and the shell's own Box is a ROW
            flex container. With one child that never showed; a second would
            sit beside the page at half width. Stack explicitly, hand the
            shell one item. */}
          <Box
            sx={{
              display: 'flex',
              flex: 1,
              flexDirection: 'column',
              // So a wide page scrolls inside itself rather than stretching
              // this column past the viewport.
              minHeight: 0,
              minWidth: 0,
            }}
          >
            {/* Headless — each returns null. Mounted inside PortProvider so a
              gate receives the live Port, and inside the shell so it runs on
              every in-shell route. */}
            {appGates.map((gate) => (
              <Fragment key={gate.id}>{gate.render(port)}</Fragment>
            ))}

            {/* Above the page, below the header: a user meeting a page of
              disabled controls needs the one sentence that explains why. The
              content is the supplier's own markup; this portal writes none of
              it. Never rendered when nothing reports read-only. */}
            {readOnly && <ReadOnlyBanner content={readOnlyBanner ?? readOnlyMessage} />}

          {/* Pages render their own container (`AppPage`, or `PageContent`
            directly), so the shell adds none and a page owns its width and
            padding. The fallbacks stand in for a page, so they bring one of
            their own. */}
            <Box sx={{ flex: 1, minWidth: 0, width: '100%', p: 1 }}>
              {/* Error boundary scoped to routed page only; resets on pathname change */}
              <ErrorBoundary
                fallback={(error, reset) => <PageErrorFallback error={error} reset={reset} />}
                resetKeys={[location.pathname]}
              >
                <Suspense
                  fallback={
                    <AppPage>
                      <LoadingState label="Loading" />
                    </AppPage>
                  }
                >
                  <Outlet />
                </Suspense>
              </ErrorBoundary>
            </Box>
          </Box>
        </AppShell.Main>

        <AppShell.Footer>
          {/* id is an anchor for measuring the footer height so sticky action
            bars (develop tabs' SaveBar) can offset above it — see SaveBar. */}
          <Box id={APP_FOOTER_ID}>
            <Footer>
              <Footer.Copyright>© {new Date().getFullYear()} WSO2 LLC.</Footer.Copyright>
              {/* Show the environment label only for pinned on-prem builds. */}
              {runtimeConfig.deploymentMode === 'onprem' && (
                <Footer.Version>{runtimeConfig.environmentName}</Footer.Version>
              )}
              <Footer.Link href={runtimeConfig.termsOfUseLink}>
                <FormattedMessage
                  id="appLayout.footer.termsOfUse"
                  defaultMessage="Terms of Use"
                  description="Footer link to the Terms of Use page"
                />
              </Footer.Link>
              <Footer.Link href={runtimeConfig.privacyPolicyLink}>
                <FormattedMessage
                  id="appLayout.footer.privacyPolicy"
                  defaultMessage="Privacy Policy"
                  description="Footer link to the Privacy Policy page"
                />
              </Footer.Link>
            </Footer>
          </Box>
        </AppShell.Footer>

        <AppShell.NotificationPanel>
          <NotificationPanel>
            <NotificationPanel.Header>
              <NotificationPanel.HeaderIcon>
                <Bell size={18} />
              </NotificationPanel.HeaderIcon>
              <NotificationPanel.HeaderTitle>
                <FormattedMessage
                  id="appLayout.notificationPanel.headerTitle"
                  defaultMessage="Notifications"
                  description="Header title for the notification panel"
                />
              </NotificationPanel.HeaderTitle>
              <NotificationPanel.HeaderClose />
            </NotificationPanel.Header>
            <NotificationPanel.EmptyState message="You're all caught up." />
          </NotificationPanel>
        </AppShell.NotificationPanel>
      </AppShell>
    </PortProvider>
  );
}
