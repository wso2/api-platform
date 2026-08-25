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

import {
  AppBreadcrumbs,
  AppShell,
  Box,
  Footer,
  NotificationPanel,
  PageContent,
  Stack,
} from '@wso2/oxygen-ui';
import type { BreadcrumbItem } from '@wso2/oxygen-ui';
import { Bell } from '@wso2/oxygen-ui-icons-react';
import { Suspense } from 'react';
import { matchPath, Outlet, useLocation, useNavigate } from 'react-router-dom';

import { LoadingState } from '../../components/StateViews';
import { runtimeConfig } from '../../config/runtime';
import { routes } from '../../routes/paths';
import { useConsoleScope } from '../../scope/ConsoleScopeProvider';
import { useNotifications } from '../../components/Notifications';
import { PortProvider, type CloudHostPort } from '../../hostPort';
import { AppHeader } from './AppHeader';
import { APP_FOOTER_ID } from './appLayoutConstants';
import { AppSidebar } from './AppSidebar';
import { FormattedMessage } from 'react-intl';

/**
 * Full-page creation flows, which the shell renders without a breadcrumb trail.
 *
 * A wizard is creating the very scope a trail would describe, so the crumbs can
 * only point at where the user came from — noise beside a form that owns the
 * whole page. Built from the route builders rather than written out, so a path
 * change cannot silently stop matching (`routes.*` is the single source).
 */
const BREADCRUMB_FREE_ROUTES = [routes.newApi(), routes.newGateway()];

export default function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { organization, project, component, params } = useConsoleScope();
  const { notify } = useNotifications();

  const hidesBreadcrumbs = BREADCRUMB_FREE_ROUTES.some(
    (path) => matchPath(path, location.pathname) !== null
  );

  // Built once per render from this portal's own hooks, then handed down as
  // a plain value to every extension's `render(port)` — see `hostPort.tsx`
  // for why this crosses the api-platform/apim-saas seam as a value, not a
  // shared context object.
  const port: CloudHostPort = {
    orgHandle: params.orgHandle ?? '',
    projectHandle: params.projectHandler,
    navigate,
    notify,
  };

  const crumbs: BreadcrumbItem[] = [];
  if (params.orgHandle) {
    crumbs.push({
      key: 'org',
      label: organization?.displayName || params.orgHandle,
      onClick: () => navigate(routes.organizationHome(params.orgHandle!)),
    });
  }
  if (params.orgHandle && params.projectHandler) {
    crumbs.push({
      key: 'project',
      label: project?.displayName || params.projectHandler,
      onClick: () =>
        navigate(routes.projectHome(params.orgHandle!, params.projectHandler!)),
    });
  }
  if (params.orgHandle && params.projectHandler && params.apiHandler) {
    crumbs.push({
      key: 'api',
      label: component?.displayName || params.apiHandler,
      onClick: () =>
        navigate(
          routes.api(
            params.orgHandle!,
            params.projectHandler!,
            params.apiHandler!
          )
        ),
    });
  }
  // The final crumb is the current page — render it as plain text (no nav).
  const breadcrumbItems = crumbs.map((crumb, index) =>
    index === crumbs.length - 1 ? { ...crumb, onClick: undefined } : crumb
  );

  return (
    <PortProvider value={port}>
      <AppShell initialCollapsed={false} collapseOnSelectOnMobile>
        <AppShell.Navbar>
          <AppHeader />
        </AppShell.Navbar>

        <AppShell.Sidebar>
          <AppSidebar />
        </AppShell.Sidebar>

      <AppShell.Main>
        <Box sx={{ minWidth: 0, width: '100%', p: 1 }}>
          
          <Suspense fallback={<LoadingState label="Loading" />}>
            <PageContent fullWidth>
                <Stack spacing={1}>
                    {!hidesBreadcrumbs && breadcrumbItems.length > 1 && (
                      <AppBreadcrumbs items={breadcrumbItems} />
                    )}
                  <Outlet />
                </Stack>
            </PageContent>
          </Suspense>
        </Box>
      </AppShell.Main>

      <AppShell.Footer>
        {/* id is an anchor for measuring the footer height so sticky action
            bars (develop tabs' SaveBar) can offset above it — see SaveBar. */}
        <Box id={APP_FOOTER_ID}>
          <Footer>
            <Footer.Copyright>
              © {new Date().getFullYear()} WSO2 LLC.
            </Footer.Copyright>
            <Footer.Version>{runtimeConfig.environmentName}</Footer.Version>
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
