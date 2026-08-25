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
  Box,
  ColorSchemeToggle,
  ComplexSelect,
  Header,
  IconButton,
  Tooltip,
  UserMenu,
  useAppShell,
} from '@wso2/oxygen-ui';
import { Bell, Boxes, Building, Layers, LogOut, WSO2, X } from '@wso2/oxygen-ui-icons-react';
import { useNavigate } from 'react-router-dom';

import { useRestApis } from '../../api/resources/restApis';
import { useAuth } from '../../contexts/auth/AuthProvider';
import { routes } from '../../routes/paths';
import { useConsoleScope } from '../../scope/ConsoleScopeProvider';
import { FormattedMessage, useIntl } from 'react-intl';
import ProjectQuickSelector from './ProjectQuickSelector';
import APIQuickSelector from './APIQuickSelector';
import SearchableComplexSelect from '../../components/common/SearchableComplexSelect';

// Switcher options can carry long display names/handles; bound the trigger width
// and let the option text ellipsize instead of overflowing the header.
const SWITCHER_SELECT_SX = { minWidth: 220, maxWidth: 260 };

const TRUNCATED_OPTION_TEXT_SLOT_PROPS = {
  primary: { noWrap: true },
  secondary: { variant: 'caption' as const, noWrap: true },
};

export function AppHeader() {
  const navigate = useNavigate();
  const intl = useIntl();
  const { actions } = useAppShell();
  const { component, organization, organizations, params, project, projects, isLoading, projectsError } =
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

  const changeApi = (apiHandler: string) => {
    if (!params.orgHandle || !params.projectHandler || !apiHandler) return;
    navigate(
      routes.api(params.orgHandle, params.projectHandler, apiHandler)
    );
  }

  const clearProjectSelection = () => {
    if (!params.orgHandle) return;
    navigate(routes.organizationHome(params.orgHandle));
  };

  const clearApiSelection = () => {
    if (!params.orgHandle || !params.projectHandler) return;
    navigate(routes.projectHome(params.orgHandle, params.projectHandler));
  };

  // organizations may not be loaded yet on first paint; keep the current org
  // selectable so the switcher never renders an out-of-range value.
  const orgOptions: { handle: string; name: string }[] =
    organizations.length > 0
      ? organizations.map((org) => ({ handle: org.id, name: org.displayName || org.id }))
      : params.orgHandle
        ? [{ handle: params.orgHandle, name: organization?.displayName || params.orgHandle }]
        : [];

  const projectOptions: { handler: string; name: string }[] =
    projects.length > 0 
      ? projects.map((project) => ({ handler: project.id, name: project.displayName || project.id })) 
      : params.projectHandler && project ? [{ handler: params.projectHandler, name: project.displayName || params.projectHandler }] 
        : [];

  const apisQuery = useRestApis(
    {},
    { projectId: project?.id, orgId: organization?.id }
  );
  const apis = apisQuery.data?.list ?? [];
  const loadedApiOptions: { handler: string; name: string }[] =
    (projects.length > 0 && project) ?
    apis
      .filter(
        (
          api
        ): api is NonNullable<typeof api> & { id: string; displayName?: string } =>
          typeof api?.id === 'string' && api.id.length > 0
      )
      .map((api) => ({ handler: api.id, name: api.displayName ?? api.id }))
    : [];

  // apis may not be loaded yet on first paint; keep the current API selectable
  // so the switcher never renders an out-of-range value.
  const apiOptions: { handler: string; name: string }[] =
    loadedApiOptions.length > 0
      ? loadedApiOptions
      : params.apiHandler
        ? [{ handler: params.apiHandler, name: component?.displayName || params.apiHandler }]
        : [];


  return (
    <Header>
      <Header.Toggle />
      <Header.Brand>
        <Header.BrandLogo>
          <WSO2 style={{ height: 26, width: 26 }} />
        </Header.BrandLogo>
        <Header.BrandTitle>
          <FormattedMessage
            id="appShell.header.title"
            defaultMessage="API Platform"
          />
        </Header.BrandTitle>
      </Header.Brand>

      {params.orgHandle && (
        <Header.Switchers showDivider={false}>
          <SearchableComplexSelect
                aria-label={intl.formatMessage({ id: 'appShell.header.org.aria', defaultMessage: 'Organizations' })}
                label={intl.formatMessage({ id: 'appShell.header.org.label', defaultMessage: 'Organizations' })}
                value={params.orgHandle}
                selectedOption={orgOptions.filter((item) => item.handle === params.orgHandle).map(item => ({
                  id: item.handle,
                  handler: item.handle,
                  name: item.name,
                }))[0] || undefined}
                onChange={(id) => {
                  changeOrganization(id);
                }}
                options={orgOptions.map((item) => ({
                  id: item.handle,
                  handler: item.handle,
                  name: item.name,
                }))}
                renderOptionContent={(option) => (
                  <>
                    <ComplexSelect.MenuItem.Icon>
                      <Building size={18} />
                    </ComplexSelect.MenuItem.Icon>
                    <ComplexSelect.MenuItem.Text
                      primary={option.name}
                      secondary={option.id}
                      slotProps={TRUNCATED_OPTION_TEXT_SLOT_PROPS}
                    />
                  </>
                )}
                searchPlaceholder={intl.formatMessage({ id: 'appShell.header.org.placeholder', defaultMessage: 'Search organizations...' })}
                emptyMessage={intl.formatMessage({ id: 'appShell.header.org.empty', defaultMessage: 'No organizations found' })}
                noResultsMessage={intl.formatMessage({ id: 'appShell.header.org.noResults', defaultMessage: 'No matching organizations' })}
                sx={SWITCHER_SELECT_SX}
              />

          {params.projectHandler && (
            <Box sx={{position: 'relative'}}>
              <SearchableComplexSelect
                aria-label={intl.formatMessage({ id: 'appShell.header.project.aria', defaultMessage: 'Projects' })}
                label={intl.formatMessage({ id: 'appShell.header.project.label', defaultMessage: 'Projects' })}
                value={params.projectHandler}
                selectedOption={projectOptions.filter((item) => item.handler === params.projectHandler).map(item => ({
                  id: item.handler,
                  handler: item.handler,
                  name: item.name,
                }))[0] || undefined}
                onChange={(id) => {
                  changeProject(id);
                }}
                options={projectOptions.map((item) => ({
                  id: item.handler,
                  handler: item.handler,
                  name: item.name,
                }))}
                renderOptionContent={(option) => (
                  <>
                    <ComplexSelect.MenuItem.Icon>
                      <Layers size={18} />
                    </ComplexSelect.MenuItem.Icon>
                    <ComplexSelect.MenuItem.Text
                      primary={option.name}
                      secondary={option.id}
                      slotProps={TRUNCATED_OPTION_TEXT_SLOT_PROPS}
                    />
                  </>
                )}
                searchPlaceholder={intl.formatMessage({ id: 'appShell.header.project.placeholder', defaultMessage: 'Search projects...' })}
                emptyMessage={intl.formatMessage({ id: 'appShell.header.project.empty', defaultMessage: 'No projects found' })}
                noResultsMessage={intl.formatMessage({ id: 'appShell.header.project.noResults', defaultMessage: 'No matching projects' })}
                sx={SWITCHER_SELECT_SX}
              />

              <IconButton
                size="small"
                aria-label={intl.formatMessage({ id: 'appShell.header.project.goToOrganization', defaultMessage: 'Go to organization level' })}
                onMouseDown={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                }}
                onClick={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  clearProjectSelection();
                }}
                sx={{
                  position: "absolute",
                  top: 6,
                  right: 2,
                  zIndex: 1,
                  width: 20,
                  height: 10,
                }}
              >
                <X size={14} />
              </IconButton>
            </Box>
          )}

          {!params.projectHandler && (
            <ProjectQuickSelector
              disabled={!params.orgHandle}
              isProjectsLoading={isLoading && projectsError === undefined}
              projectsError={projectsError}
              projectOptions={projectOptions.map((item) => ({
                id: item.handler,
                handler: item.handler,
                name: item.name,
              }))}
              onSelectProject={(projectHandler) => {
                changeProject(projectHandler);
              }}
            />
          )}

          {params.apiHandler && (
            <Box sx={{position: 'relative'}}>
              <SearchableComplexSelect
                aria-label={intl.formatMessage({ id: 'appShell.header.api.aria', defaultMessage: 'APIs' })}
                label={intl.formatMessage({ id: 'appShell.header.api.label', defaultMessage: 'APIs' })}
                value={params.apiHandler}
                selectedOption={apiOptions.filter((item) => item.handler === params.apiHandler).map(item => ({
                  id: item.handler,
                  handler: item.handler,
                  name: item.name,
                }))[0] || undefined}
                onChange={(id) => {
                  changeApi(id);
                }}
                options={apiOptions.map((item) => ({
                  id: item.handler,
                  handler: item.handler,
                  name: item.name,
                }))}
                renderOptionContent={(option) => (
                  <>
                    <ComplexSelect.MenuItem.Icon>
                      <Boxes size={18} />
                    </ComplexSelect.MenuItem.Icon>
                    <ComplexSelect.MenuItem.Text
                      primary={option.name}
                      secondary={option.id}
                      slotProps={TRUNCATED_OPTION_TEXT_SLOT_PROPS}
                    />
                  </>
                )}
                searchPlaceholder={intl.formatMessage({ id: 'appShell.header.api.placeholder', defaultMessage: 'Search APIs...' })}
                emptyMessage={intl.formatMessage({ id: 'appShell.header.api.empty', defaultMessage: 'No APIs found' })}
                noResultsMessage={intl.formatMessage({ id: 'appShell.header.api.noResults', defaultMessage: 'No matching APIs' })}
                sx={SWITCHER_SELECT_SX}
              />

              <IconButton
                size="small"
                aria-label={intl.formatMessage({ id: 'appShell.header.api.goToProjectLevel', defaultMessage: 'Go to project level' })}
                onMouseDown={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                }}
                onClick={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  clearApiSelection();
                }}
                sx={{
                  position: "absolute",
                  top: 6,
                  right: 2,
                  zIndex: 1,
                  width: 20,
                  height: 10,
                }}
              >
                <X size={14} />
              </IconButton>
            </Box>
          )}

          {!params.apiHandler && params.projectHandler && (
            <APIQuickSelector
              disabled={!params.projectHandler}
              isApisLoading={apisQuery.isLoading}
              apisError={apisQuery.error}
              apiOptions={apiOptions.map((item) => ({
                id: item.handler,
                handler: item.handler,
                name: item.name,
              }))}
              onSelectApi={(apiHandler) => {
                changeApi(apiHandler);
              }}
            />
          )}
        </Header.Switchers>
      )}

      <Header.Spacer />

      <Header.Actions>
        <ColorSchemeToggle />
        <Tooltip title={intl.formatMessage({ id: 'appShell.header.notifications', defaultMessage: 'Notifications' })}>
          <IconButton
            aria-label={intl.formatMessage({ id: 'appShell.header.notifications', defaultMessage: 'Notifications' })}
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
