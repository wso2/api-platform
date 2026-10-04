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
import { Suspense } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

import { AppPage } from '../../components/AppPage';
import { ErrorBoundary } from '../../components/errors/ErrorBoundary';
import { PageErrorFallback, SidebarErrorFallback } from '../../components/errors/ErrorFallback';
import { LoadingState } from '../../components/StateViews';
import { runtimeConfig } from '../../config/runtime';
import { useDocumentTitle } from '../../hooks/useDocumentTitle';
import { usePageTitle } from '../../navigation/usePageTitle';
import { useConsoleScope } from '../../scope/ConsoleScopeProvider';
import { useNotifications } from '../../components/Notifications';
import { extensionApiFetch, PortProvider, type CloudHostPort } from '../../hostPort';
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
          {/* Pages render their own container (`AppPage`, or `PageContent`
            directly), so the shell adds none and a page owns its width and
            padding. The fallbacks stand in for a page, so they bring one of
            their own. */}
          <Box sx={{ minWidth: 0, width: '100%', p: 1 }}>
            {/* Error boundary scoped to routed page only; resets on pathname change */}
            <ErrorBoundary
              fallback={(error, reset) => <PageErrorFallback error={error} reset={reset} />}
              resetKeys={[location.pathname]}
            >
              <Suspense
                fallback={
                  <AppPage hideBreadcrumbs>
                    <LoadingState label="Loading" />
                  </AppPage>
                }
              >
                <Outlet />
              </Suspense>
            </ErrorBoundary>
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
