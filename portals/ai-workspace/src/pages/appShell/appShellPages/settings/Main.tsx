/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// ---------------------------------------------------------------------------
// Settings layout. A persistent left sub-nav (always visible) + a vertical
// separator; everything else — the templates list, create, overview, edit and
// create-version pages — renders in the right pane via <Outlet />. The first
// nav item is LLM Provider Templates. Add new items to NAV_ITEMS to extend.
// ---------------------------------------------------------------------------

import { type ReactNode } from 'react';
import { Navigate, Outlet, useNavigate } from 'react-router-dom';
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
import { LayoutTemplate } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { SCOPES } from '../../../../auth/permissions';
import { buildOrgPath } from '../../../../utils/projectRouting';

interface NavItem {
  key: string;
  label: string;
  icon: ReactNode;
  // Settings-relative path the item navigates to.
  path: string;
}

// Settings navigation. LLM Provider Templates is first/default.
const NAV_ITEMS: NavItem[] = [
  {
    key: 'templates',
    label: 'LLM Provider Templates',
    icon: <LayoutTemplate size={18} />,
    path: '/settings/llm-provider-templates',
  },
];

export default function Settings() {
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();
  const { hasPermission } = useAppAuth();
  // Only one area today; it stays selected across all its sub-pages.
  const selectedKey = 'templates';

  // Settings requires template-manage permission; send others to org home.
  if (!hasPermission(SCOPES.LLM_TEMPLATE_MANAGE)) {
    return (
      <Navigate
        to={buildOrgPath(currentOrganization, '/home')}
        replace
      />
    );
  }

  return (
    <Box sx={{ display: 'flex', alignItems: 'stretch', minHeight: '100%', width: '100%' }}>
      {/* Persistent left sub-nav */}
      <Box sx={{ width: { xs: 200, md: 280 }, flexShrink: 0, p: 3 }}>
        <Stack spacing={2}>
          <PageTitle>
            <PageTitle.Header>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.settings.Main.title"
                defaultMessage={'Settings'}
              />
            </PageTitle.Header>
          </PageTitle>
          <List dense disablePadding>
            {NAV_ITEMS.map((item) => (
              <ListItemButton
                key={item.key}
                selected={item.key === selectedKey}
                onClick={() => navigate(buildOrgPath(currentOrganization, item.path))}
                sx={{
                  borderRadius: 1,
                  mb: 0.5,
                  border: 1,
                  borderColor: 'divider',
                }}
              >
                <ListItemIcon sx={{ minWidth: 32 }}>{item.icon}</ListItemIcon>
                <ListItemText
                  primary={item.label}
                  slotProps={{ primary: { noWrap: true } }}
                />
              </ListItemButton>
            ))}
          </List>
        </Stack>
      </Box>

      <Divider orientation="vertical" flexItem />

      {/* Right pane — the active settings page renders here. */}
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Outlet />
      </Box>
    </Box>
  );
}
