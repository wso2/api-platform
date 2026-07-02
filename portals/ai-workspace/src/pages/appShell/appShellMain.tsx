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

import type { JSX } from 'react';
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
// import { useAuthContext } from '@asgardeo/auth-react'; // [standalone]
import {
  AppShell,
  Button,
  Footer,
  useAppShell as useOxygenAppShell,
  useNotifications,
  Box,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';

// import { handleLogout } from '../../auth/logout'; // [standalone]
import { useAppShell as useWorkspaceAppShell } from '../../contexts/AppShellContext';
import { useAppAuth } from '../../contexts/AppAuthContext';

import AppHeader from './AppHeader';
import AppSidebar from './AppSidebar';
import AppNotifications from './AppNotifications';
import AILoader from '../../Components/AILoader';
import OrgProvisioningPage from '../register/OrgProvisioningPage';
import {
  getOrgSlug,
  getProjectSlug,
  buildOrgPath,
} from '../../utils/projectRouting';
import { logger } from '../../utils/logger';
import { FormattedMessage } from 'react-intl';
import OoopsImage from '../../assets/images/Ooops.svg';

type SelectableOrg = {
  id: string;
  name: string;
  description?: string;
};

type SelectableProject = {
  id: string;
  name: string;
  description?: string;
};

export default function AppLayout(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const { logout } = useAppAuth();
  // [standalone] const { signOut } = useAuthContext();

  const {
    userName,
    userEmail,

    currentOrganization,

    projectsForCurrentOrganization,
    currentProject,
    setCurrentProject,
    isProjectsLoading,

    isLoading,
    isProvisioning,
    provisioningOrgName,
    error,
  } = useWorkspaceAppShell();

  const onLogout = useCallback(() => { void logout(); }, [logout]);

  const { state: shellState, actions: shellActions } = useOxygenAppShell({
    initialCollapsed: false,
  });

  const {
    notifications,
    actions: notifActions,
    unreadCount,
    unreadNotifications,
  } = useNotifications({
    initialNotifications: [],
  });

  const [tabIndex, setTabIndex] = useState(0);

  const projectOptions: SelectableProject[] = useMemo(() => {
    return Array.isArray(projectsForCurrentOrganization)
      ? projectsForCurrentOrganization.map((project) => ({
          id: project.id,
          name: project.displayName,
          description: project.description,
        }))
      : [];
  }, [projectsForCurrentOrganization]);

  const currentProjectOption: SelectableProject | null = useMemo(
    () => projectOptions.find((project) => project.id === currentProject?.id) ?? null,
    [projectOptions, currentProject?.id]
  );

  const [selectedProjectId, setSelectedProjectId] = useState<string>(
    currentProject?.id ?? ''
  );

  useEffect(() => {
    setSelectedProjectId(currentProject?.id ?? '');
  }, [currentProject?.id]);

  useEffect(() => {
    if (error) {
      logger.error('App shell error state:', error);
    }
  }, [error]);

  // ✅ Sidebar active item sync
  useEffect(() => {
    const segments = location.pathname.split('/').filter(Boolean);
    const orgSlug = getOrgSlug(currentOrganization);
    const projectSlug = getProjectSlug(currentProject);
    const isOrgScoped =
      segments[0] === 'organizations' && segments[1] === orgSlug;
    const primarySegment = isOrgScoped ? segments[2] ?? '' : segments[0] ?? '';
    const secondarySegment = isOrgScoped
      ? segments[3] ?? ''
      : segments[1] ?? '';
    const tertiarySegment = isOrgScoped ? segments[4] ?? '' : segments[2] ?? '';

    if (!primarySegment || primarySegment === 'home') {
      shellActions.setActiveMenuItem('overview');
      return;
    }
    if (primarySegment === 'quick-start') {
      shellActions.setActiveMenuItem('quick-start');
      return;
    }
    if (primarySegment === 'projects') {
      if (!secondarySegment) {
        shellActions.setActiveMenuItem('projects');
        return;
      }
      if (secondarySegment !== projectSlug) {
        shellActions.setActiveMenuItem('projects');
        return;
      }
      if (tertiarySegment === 'home') {
        shellActions.setActiveMenuItem('overview');
        return;
      }
      if (tertiarySegment === 'applications') {
        shellActions.setActiveMenuItem('applications');
        return;
      }
      if (tertiarySegment === 'proxies') {
        shellActions.setActiveMenuItem('proxies');
        return;
      }
      if (tertiarySegment === 'service-provider') {
        shellActions.setActiveMenuItem('service-provider');
        return;
      }
      if (tertiarySegment === 'provider-template') {
        shellActions.setActiveMenuItem('provider-template');
        return;
      }
      if (tertiarySegment === 'external-servers') {
        shellActions.setActiveMenuItem('external-servers');
        return;
      }
      if (tertiarySegment === 'registries') {
        shellActions.setActiveMenuItem('registries');
        return;
      }
      if (tertiarySegment === 'gateways') {
        shellActions.setActiveMenuItem('gateways');
        return;
      }
      if (tertiarySegment === 'insights') {
        shellActions.setActiveMenuItem('insights');
        return;
      }
      if (tertiarySegment === 'quick-start') {
        shellActions.setActiveMenuItem('quick-start');
        return;
      }
      if (tertiarySegment === 'settings') {
        shellActions.setActiveMenuItem('settings');
      }
      return;
    }
    if (primarySegment === 'applications') {
      shellActions.setActiveMenuItem('applications');
      return;
    }
    if (primarySegment === 'proxies') {
      shellActions.setActiveMenuItem('proxies');
      return;
    }
    if (primarySegment === 'service-provider') {
      shellActions.setActiveMenuItem('service-provider');
      return;
    }
    if (primarySegment === 'provider-template') {
      shellActions.setActiveMenuItem('provider-template');
      return;
    }
    if (primarySegment === 'external-servers') {
      shellActions.setActiveMenuItem('external-servers');
      return;
    }
    if (primarySegment === 'registries') {
      shellActions.setActiveMenuItem('registries');
      return;
    }
    if (primarySegment === 'gateways') {
      shellActions.setActiveMenuItem('gateways');
      return;
    }
    if (primarySegment === 'insights') {
      shellActions.setActiveMenuItem('insights');
      return;
    }
    if (primarySegment === 'settings') {
      shellActions.setActiveMenuItem('settings');
    }
  }, [location.pathname, shellActions, currentProject, currentOrganization]);

  useEffect(() => {
    const legalLinks = [
      'https://wso2.com/bijira/terms-of-use',
      'https://wso2.com/bijira/privacy-policy',
    ];

    legalLinks.forEach((url) => {
      document
        .querySelectorAll<HTMLAnchorElement>(`a[href="${url}"]`)
        .forEach((anchor) => {
          anchor.target = '_blank';
          anchor.rel = 'noopener noreferrer';
        });
    });
  }, [location.pathname]);

  if (isProvisioning) {
    return (
      <OrgProvisioningPage
        orgName={provisioningOrgName ?? undefined}
        isProvisioning
      />
    );
  }

  if (isLoading) {
    return (
      <Box
        sx={{
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          px: 2,
        }}
      >
        <Box
          sx={{
            width: 'min(520px, 100%)',
            textAlign: 'center',
          }}
        >
          <Stack spacing={2.5} alignItems="center">
            <Box>
              <AILoader />
            </Box>

            <Typography variant="body2">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellMain.loading.organizations"
                defaultMessage={'Loading Organizations...'}
              />
            </Typography>
          </Stack>
        </Box>
      </Box>
    );
  }

  if (error) {
    return (
      <Box
        sx={{
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          px: 2,
        }}
      >
        <Box
          sx={{
            width: 'min(560px, 100%)',
            textAlign: 'center',
          }}
        >
          <Stack spacing={1} alignItems="center">
            <Box
              component="img"
              src={OoopsImage}
              alt="Oops"
              sx={{
                width: 180,
                maxWidth: '100%',
                height: 'auto',
              }}
            />
            <Typography
              variant="h4"
              sx={{
                fontWeight: 700,
              }}
            >
              Oops! This is embarrassing
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Something went terribly wrong. Will you please refresh and try
              again.
            </Typography>
            <Button variant="contained" onClick={onLogout}>
              Logout
            </Button>
          </Stack>
        </Box>
      </Box>
    );
  }

  return (
    <AppShell>
      <AppShell.Navbar>
        <AppHeader
          shellState={shellState}
          shellActions={shellActions}
          navigate={navigate}
          userName={userName ?? undefined}
          userEmail={userEmail ?? undefined}
          unreadCount={unreadCount}
          currentOrganization={
            currentOrganization
              ? {
                  id: String(currentOrganization.id),
                  name: currentOrganization.name,
                  handle: currentOrganization.handle,
                }
              : null
          }
          projectOptions={projectOptions}
          currentProject={currentProjectOption}
          setCurrentProject={(p) => {
            if (!p) {
              setCurrentProject(null);
              return;
            }
            const matchedProject = projectsForCurrentOrganization.find(
              (proj) => proj.id === p.id
            );
            if (matchedProject) {
              setCurrentProject(matchedProject);
            }
          }}
          isProjectsLoading={isProjectsLoading}
          selectedProjectId={selectedProjectId}
          setSelectedProjectId={setSelectedProjectId}
          onLogout={onLogout}
        />
      </AppShell.Navbar>

      <AppShell.Sidebar>
        <AppSidebar
          shellState={shellState}
          shellActions={shellActions}
          projectCount={projectOptions.length}
        />
      </AppShell.Sidebar>

      <AppShell.Main>
        <Outlet />
      </AppShell.Main>

      <AppShell.Footer>
        <Box />
        <Footer>
          <Footer.Copyright>
            © {new Date().getFullYear()} WSO2 LLC. All rights reserved.
          </Footer.Copyright>
          <Footer.Divider />
          <Footer.Version>AI Workspace</Footer.Version>
          <Footer.Link
            href="https://wso2.com/bijira/terms-of-use"
            target="_blank"
            rel="noopener noreferrer"
          >
            Terms & Conditions
          </Footer.Link>
          <Footer.Link
            href="https://wso2.com/bijira/privacy-policy"
            target="_blank"
            rel="noopener noreferrer"
          >
            Privacy Policy
          </Footer.Link>
        </Footer>
      </AppShell.Footer>

      <AppShell.NotificationPanel>
        <AppNotifications
          shellState={shellState}
          shellActions={shellActions}
          notifications={notifications}
          unreadCount={unreadCount}
          unreadNotifications={unreadNotifications}
          notifActions={notifActions}
          tabIndex={tabIndex}
          setTabIndex={setTabIndex}
          onLogout={onLogout}
          navigate={navigate}
        />
      </AppShell.NotificationPanel>
    </AppShell>
  );
}
