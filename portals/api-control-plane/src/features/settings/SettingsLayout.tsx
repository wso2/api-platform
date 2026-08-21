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

// Settings layout. A persistent left sub-nav (built-in tabs plus any
// cloud-injected `settingsTab` extension, see `useSettingsTabs`) + a vertical
// divider; the active tab renders in the right pane via <Outlet />.

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
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

import { useConsoleScope } from '../../scope/ConsoleScopeProvider';
import { routes } from '../../routes/paths';
import { useSettingsTabs } from '../../navigation/useSettingsTabs';
import type { NavigationLevel } from '../../navigation/navigationTypes';

export type SettingsLayoutProps = {
  /** Which Settings page this is — organization- or project-scoped. */
  scope: Extract<NavigationLevel, 'organization' | 'project'>;
};

export function SettingsLayout({ scope }: SettingsLayoutProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { params } = useConsoleScope();
  const tabs = useSettingsTabs(scope);

  const selectedId = tabs.find((tab) =>
    location.pathname.endsWith(`/settings/${tab.path}`)
  )?.id;

  const goToTab = (path: string) => {
    if (!params.orgHandle) return;
    if (scope === 'project') {
      if (!params.projectHandler) return;
      navigate(routes.settingsTab(path, params.orgHandle, params.projectHandler));
    } else {
      navigate(routes.orgSettingsTab(path, params.orgHandle));
    }
  };

  return (
    <Box sx={{ display: 'flex', alignItems: 'stretch', minHeight: '100%', width: '100%' }}>
      <Box sx={{ width: { xs: 200, md: 280 }, flexShrink: 0, p: 3 }}>
        <Stack spacing={2}>
          <PageTitle>
            <PageTitle.Header>Settings</PageTitle.Header>
          </PageTitle>
          <List dense disablePadding>
            {tabs.map((tab) => (
              <ListItemButton
                key={tab.id}
                selected={tab.id === selectedId}
                onClick={() => goToTab(tab.path)}
                sx={{ borderRadius: 1, mb: 0.5, border: 1, borderColor: 'divider' }}
              >
                <ListItemIcon sx={{ minWidth: 32 }}>{tab.icon}</ListItemIcon>
                <ListItemText primary={tab.label} slotProps={{ primary: { noWrap: true } }} />
              </ListItemButton>
            ))}
          </List>
        </Stack>
      </Box>

      <Divider orientation="vertical" flexItem />

      <Box sx={{ flex: 1, minWidth: 0, p: 3 }}>
        <Outlet />
      </Box>
    </Box>
  );
}
