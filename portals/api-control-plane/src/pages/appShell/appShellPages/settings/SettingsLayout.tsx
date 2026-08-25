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
  Box,
  Divider,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  PageTitle,
  Stack,
} from '@wso2/oxygen-ui';
import { defineMessages, useIntl } from 'react-intl';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

import { useConsoleScope } from '../../../../scope/ConsoleScopeProvider';
import { routes } from '../../../../routes/paths';
import { useSettingsTabs } from '../../../../navigation/useSettingsTabs';
import type { NavigationLevel } from '../../../../navigation/navigationTypes';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SettingsLayout.title',
    defaultMessage: 'Settings',
    description: 'Heading of the Settings page.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.SettingsLayout.subtitle',
    defaultMessage: 'Minimal settings overview for {subject}.',
    description:
      'Sub-heading of the Settings page. {subject} is the display name of the organization or project being configured — never translated.',
  },
});

export type SettingsLayoutProps = {
  /** Which Settings page this is — organization- or project-scoped. */
  level: Extract<NavigationLevel, 'organization' | 'project'>;
};

/**
 * Settings shell: a persistent left sub-nav (the built-in tabs plus any
 * host-injected `settings.<level>.tabs` extension, see `useSettingsTabs`) and
 * the active tab in the right pane via `<Outlet />`.
 *
 * No `ScopeGate`: Settings is the one page with no scope requirement. The
 * sidebar links to the organization-level path while browsing the org and to
 * the project's once one is selected, and a project card's gear deep-links the
 * same page — so it renders at whatever scope it is reached in.
 */
export function SettingsLayout({ level }: SettingsLayoutProps) {
  const intl = useIntl();
  const navigate = useNavigate();
  const location = useLocation();
  const { organization, params, project } = useConsoleScope();
  const tabs = useSettingsTabs(level);

  // Whichever scope the page was reached in, named rather than handled: the
  // heading reads "…for Retail APIs", not "…for retail-apis". Falls back to the
  // handle, which the route always carries, so the heading still says what it is
  // about while the display name is still loading.
  const subject =
    project?.displayName ??
    organization?.displayName ??
    params.projectHandler ??
    params.orgHandle;

  // The index route carries no tab segment, and renders the first tab's
  // content — so it highlights the first tab rather than nothing at all.
  const selectedId =
    tabs.find((tab) => location.pathname.endsWith(`/settings/${tab.path}`))?.id ??
    tabs[0]?.id;

  const goToTab = (path: string) => {
    if (!params.orgHandle) return;
    if (level === 'project') {
      if (!params.projectHandler) return;
      navigate(
        routes.projectSettingsTab(path, params.orgHandle, params.projectHandler)
      );
      return;
    }
    navigate(routes.settingsTab(path, params.orgHandle));
  };

  return (
    <Box
      sx={{
        alignItems: 'stretch',
        display: 'flex',
        minHeight: '100%',
        width: '100%',
      }}
    >
      <Box sx={{ flexShrink: 0, pr: 3, width: { md: 280, xs: 200 } }}>
        <Stack spacing={2}>
          <PageTitle>
            <PageTitle.Header>
              {intl.formatMessage(messages.title)}
            </PageTitle.Header>
            <PageTitle.SubHeader>
              {intl.formatMessage(messages.subtitle, { subject })}
            </PageTitle.SubHeader>
          </PageTitle>
          <List dense disablePadding>
            {tabs.map((tab) => (
              <ListItemButton
                key={tab.id}
                onClick={() => goToTab(tab.path)}
                selected={tab.id === selectedId}
                sx={{
                  borderColor: 'divider',
                  borderRadius: 1,
                  border: 1,
                  mb: 0.5,
                }}
              >
                <ListItemIcon sx={{ minWidth: 32 }}>{tab.icon}</ListItemIcon>
                <ListItemText
                  primary={tab.label}
                  slotProps={{ primary: { noWrap: true } }}
                />
              </ListItemButton>
            ))}
          </List>
        </Stack>
      </Box>

      <Divider orientation="vertical" flexItem />

      <Box sx={{ flex: 1, minWidth: 0, pl: 3 }}>
        <Outlet />
      </Box>
    </Box>
  );
}
