import {
  AppBreadcrumbs,
  AppShell,
  Box,
  Footer,
  NotificationPanel,
} from '@wso2/oxygen-ui';
import type { BreadcrumbItem } from '@wso2/oxygen-ui';
import { Bell } from '@wso2/oxygen-ui-icons-react';
import { Suspense } from 'react';
import { Outlet, useNavigate } from 'react-router-dom';

import { LoadingState } from '../components/StateViews';
import { runtimeConfig } from '../config/runtime';
import { routes } from '../routes/paths';
import { useConsoleScope } from '../scope/ConsoleScopeProvider';
import { AppHeader } from './AppHeader';
import { APP_FOOTER_ID } from './appLayoutConstants';
import { AppSidebar } from './AppSidebar';

export default function AppLayout() {
  const navigate = useNavigate();
  const { organization, project, component, params } = useConsoleScope();

  const crumbs: BreadcrumbItem[] = [];
  if (params.orgHandle) {
    crumbs.push({
      key: 'org',
      label: organization?.name || params.orgHandle,
      onClick: () => navigate(routes.organizationHome(params.orgHandle!)),
    });
  }
  if (params.orgHandle && params.projectHandler) {
    crumbs.push({
      key: 'project',
      label: project?.name || params.projectHandler,
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
    <AppShell initialCollapsed={false} collapseOnSelectOnMobile>
      <AppShell.Navbar>
        <AppHeader />
      </AppShell.Navbar>

      <AppShell.Sidebar>
        <AppSidebar />
      </AppShell.Sidebar>

      <AppShell.Main>
        <Box sx={{ minWidth: 0, width: '100%' }}>
          {breadcrumbItems.length > 1 && (
            <AppBreadcrumbs items={breadcrumbItems} sx={{ mb: 2 }} />
          )}
          <Suspense fallback={<LoadingState label="Loading" />}>
            <Outlet />
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
              Terms of Use
            </Footer.Link>
            <Footer.Link href={runtimeConfig.privacyPolicyLink}>
              Privacy Policy
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
              Notifications
            </NotificationPanel.HeaderTitle>
            <NotificationPanel.HeaderClose />
          </NotificationPanel.Header>
          <NotificationPanel.EmptyState message="You're all caught up." />
        </NotificationPanel>
      </AppShell.NotificationPanel>
    </AppShell>
  );
}
