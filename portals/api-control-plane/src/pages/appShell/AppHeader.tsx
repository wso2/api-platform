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
  Badge,
  ColorSchemeToggle,
  ComplexSelect,
  Header,
  IconButton,
  Tooltip,
  UserMenu,
  useAppShell,
} from '@wso2/oxygen-ui';
import { Bell, Boxes, Building2, LogOut, WSO2 } from '@wso2/oxygen-ui-icons-react';
import { useNavigate } from 'react-router-dom';

import { useAuth } from '../../contexts/auth/AuthProvider';
import { routes } from '../../routes/paths';
import { useConsoleScope } from '../../scope/ConsoleScopeProvider';

export function AppHeader() {
  const navigate = useNavigate();
  const { actions } = useAppShell();
  const { organization, organizations, params, project, projects } =
    useConsoleScope();
  const auth = useAuth();

  const userName = auth.user?.name || 'User';
  const userEmail = auth.user?.email || '';

  const changeOrganization = (orgHandle: string) => {
    if (!orgHandle || orgHandle === params.orgHandle) return;
    navigate(routes.organizationHome(orgHandle));
  };

  const changeProject = (projectHandler: string) => {
    if (!params.orgHandle || !projectHandler) return;
    navigate(routes.projectHome(params.orgHandle, projectHandler));
  };

  // organizations may not be loaded yet on first paint; keep the current org
  // selectable so the switcher never renders an out-of-range value.
  const orgOptions: { handle: string; name: string }[] =
    organizations.length > 0
      ? organizations
      : params.orgHandle
        ? [{ handle: params.orgHandle, name: organization?.name || params.orgHandle }]
        : [];
  const projectOptions: { handler: string; name: string }[] =
    projects.length > 0 ? projects : project ? [project] : [];

  return (
    <Header>
      <Header.Toggle />
      <Header.Brand>
        <Header.BrandLogo>
          <WSO2 style={{ height: 26, width: 26 }} />
        </Header.BrandLogo>
        <Header.BrandTitle>API Platform</Header.BrandTitle>
      </Header.Brand>

      {params.orgHandle && (
        <Header.Switchers showDivider>
          <ComplexSelect
            aria-label="Organization"
            label="Organization"
            labelAnchor="inside"
            size="small"
            value={params.orgHandle}
            onChange={(event) => changeOrganization(String(event.target.value))}
            sx={{ minWidth: 220 }}
          >
            {orgOptions.map((item) => (
              <ComplexSelect.MenuItem key={item.handle} value={item.handle}>
                <ComplexSelect.MenuItem.Icon>
                  <Building2 size={18} />
                </ComplexSelect.MenuItem.Icon>
                <ComplexSelect.MenuItem.Text
                  primary={item.name}
                  secondary={item.handle}
                />
              </ComplexSelect.MenuItem>
            ))}
          </ComplexSelect>

          {params.projectHandler && (
            <ComplexSelect
              aria-label="Project"
              label="Project"
              labelAnchor="inside"
              size="small"
              value={params.projectHandler}
              onChange={(event) => changeProject(String(event.target.value))}
              sx={{ minWidth: 220 }}
            >
              {projectOptions.map((item) => (
                <ComplexSelect.MenuItem key={item.handler} value={item.handler}>
                  <ComplexSelect.MenuItem.Icon>
                    <Boxes size={18} />
                  </ComplexSelect.MenuItem.Icon>
                  <ComplexSelect.MenuItem.Text
                    primary={item.name}
                    secondary={item.handler}
                  />
                </ComplexSelect.MenuItem>
              ))}
            </ComplexSelect>
          )}
        </Header.Switchers>
      )}

      <Header.Spacer />

      <Header.Actions>
        <ColorSchemeToggle />
        <Tooltip title="Notifications">
          <IconButton
            aria-label="Notifications"
            onClick={actions.toggleNotificationPanel}
            size="small"
          >
            <Badge color="primary" variant="dot" invisible>
              <Bell size={20} />
            </Badge>
          </IconButton>
        </Tooltip>
        <UserMenu>
          <UserMenu.Trigger name={userName} />
          <UserMenu.Header name={userName} email={userEmail} />
          <UserMenu.Divider />
          <UserMenu.Logout icon={<LogOut size={18} />} onClick={auth.logout} />
        </UserMenu>
      </Header.Actions>
    </Header>
  );
}
