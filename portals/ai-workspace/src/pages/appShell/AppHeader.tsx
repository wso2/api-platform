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

import React, { useMemo, useState } from 'react';
import {
  Badge,
  Button,
  ColorSchemeToggle,
  ComplexSelect,
  Divider,
  Header,
  IconButton,
  Tooltip,
  Typography,
  Box,
} from '@wso2/oxygen-ui';
import { Bell, Building, X } from '@wso2/oxygen-ui-icons-react';
import SearchableComplexSelect from '../../Components/common/SearchableComplexSelect';
import Logo from '../../Components/Logo';
import UserMenu from '../../Components/UserMenu';
import { useRole } from '../../contexts/RoleContext';
import {
  buildOrgPath,
  buildProjectPath,
  getProjectSlug,
} from '../../utils/projectRouting';
import { FormattedMessage } from 'react-intl';
import ProjectQuickSelector from './ProjectQuickSelector';

type SelectableOrg = {
  id: string;
  name: string;
  description?: string;
  handle?: string;
};
type SelectableProject = {
  id: string;
  name: string;
  description?: string;
  handler?: string;
};

type Props = {
  shellState: any;
  shellActions: any;
  navigate: (to: string) => void;

  userName?: string;
  userEmail?: string;

  unreadCount?: number;

  organizationOptions: SelectableOrg[];
  currentOrganization: SelectableOrg | null;
  setCurrentOrganization?: (org: SelectableOrg | null) => void | Promise<void>;

  projectOptions: SelectableProject[];
  currentProject: SelectableProject | null;
  setCurrentProject?: (p: SelectableProject | null) => void;

  isProjectsLoading: boolean;
  projectsError?: unknown;

  selectedOrgId: string;
  setSelectedOrgId: (v: string) => void;

  selectedProjectId: string;
  setSelectedProjectId: (v: string) => void;

  onLogout?: () => void;
};

export default function AppHeader(props: Props) {
  const { role } = useRole();
  const {
    shellState,
    shellActions,
    navigate,
    userName,
    userEmail,
    unreadCount,

    organizationOptions,
    currentOrganization,
    setCurrentOrganization,

    projectOptions,
    currentProject,
    setCurrentProject,
    isProjectsLoading,
    projectsError,

    selectedOrgId,
    setSelectedOrgId,

    selectedProjectId,
    setSelectedProjectId,

    onLogout,
  } = props;

  const [isChangingOrganization, setIsChangingOrganization] = useState(false);

  const userForMenu = useMemo(
    () => ({
      name: userName || userEmail || 'User',
      email: userEmail || '',
    }),
    [userName, userEmail]
  );
  const canShowProjectSwitcher = Boolean(currentProject?.id);
  const isProjectPickerDisabled =
    isProjectsLoading || !currentOrganization?.id || isChangingOrganization;
  const handleProjectSelection = (nextProjectId: string) => {
    setSelectedProjectId(nextProjectId);

    const nextProject =
      projectOptions.find((p) => p.id === nextProjectId) ?? null;
    setCurrentProject?.(nextProject);

    if (nextProject?.id && currentOrganization?.id) {
      navigate(
        buildOrgPath(
          currentOrganization,
          `/projects/${getProjectSlug(nextProject)}/home`
        )
      );
    }
  };
  const clearProjectSelection = () => {
    setSelectedProjectId('');
    setCurrentProject?.(null);
    if (currentOrganization?.id) {
      navigate(buildOrgPath(currentOrganization, '/home'));
    }
  };
  const homePath = currentProject
    ? buildProjectPath(currentOrganization, currentProject, '/home')
    : buildOrgPath(currentOrganization, '/home');

  return (
    <Header>
      <Header.Toggle
        collapsed={shellState.sidebarCollapsed}
        onToggle={shellActions.toggleSidebar}
      />

      <Header.Brand onClick={() => navigate(homePath)} sx={{ cursor: 'pointer' }}>
        <Header.BrandLogo>
          <Logo />
        </Header.BrandLogo>
      </Header.Brand>

      <Header.Switchers showDivider={false}>
        {/* ORG SWITCHER */}
        <SearchableComplexSelect
          value={selectedOrgId}
          selectedOption={currentOrganization}
          options={organizationOptions}
          openOnFieldClick={!currentProject?.id}
          onFieldClick={currentProject?.id ? clearProjectSelection : undefined}
          fieldClickAriaLabel="Go to organization level"
          dropdownClickAriaLabel="Open organizations list"
          onChange={async (nextOrgId) => {
            setSelectedOrgId(nextOrgId);

            const nextOrg =
              organizationOptions.find((o) => o.id === nextOrgId) ?? null;

            setIsChangingOrganization(true);
            try {
              const maybePromise = setCurrentOrganization?.(nextOrg);
              if (
                maybePromise &&
                typeof (maybePromise as any).then === 'function'
              ) {
                await maybePromise;
              }
              if (nextOrg?.id) {
                navigate(buildOrgPath(nextOrg, '/home'));
              }
            } finally {
              // Small delay to avoid flicker
              await new Promise((resolve) => setTimeout(resolve, 120));
              setIsChangingOrganization(false);
            }
          }}
          sx={{ minWidth: 220 }}
          renderOptionContent={(org) => (
            <>
              <ComplexSelect.MenuItem.Icon>
                <Building />
              </ComplexSelect.MenuItem.Icon>
              <ComplexSelect.MenuItem.Text
                primary={org.name}
                secondary={org.description}
              />
            </>
          )}
          label="Organizations"
          emptyMessage="No organizations"
          noResultsMessage="No matching organizations"
          searchPlaceholder="Search organizations"
        />

        {canShowProjectSwitcher ? (
          <Box sx={{ position: 'relative', minWidth: 220 }}>
            <SearchableComplexSelect
              value={selectedProjectId}
              selectedOption={currentProject}
              options={projectOptions}
              loading={isProjectsLoading || isChangingOrganization}
              disabled={
                isProjectsLoading ||
                !currentOrganization?.id ||
                isChangingOrganization
              }
              onChange={handleProjectSelection}
              renderOptionContent={(project) => (
                <ComplexSelect.MenuItem.Text primary={project.name} />
              )}
              label="Projects"
              emptyMessage="No projects available"
              error={projectsError}
              errorMessage="Failed to load projects"
              noResultsMessage="No matching projects"
              searchPlaceholder="Search projects"
              sx={{ minWidth: 220 }}
            />
            <IconButton
              size="small"
              aria-label="Go to organization level"
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
                position: 'absolute',
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
        ) : (
          <ProjectQuickSelector
            disabled={isProjectPickerDisabled}
            isProjectsLoading={isProjectsLoading || isChangingOrganization}
            projectsError={projectsError}
            projectOptions={projectOptions}
            onSelectProject={handleProjectSelection}
          />
        )}
      </Header.Switchers>

      <Header.Spacer />

      <Header.Actions>
        {/* <Typography variant="body2" sx={{ color: 'text.secondary' }}>
          <FormattedMessage
            id="aiWorkspace.pages.appShell.AppHeader.role"
            defaultMessage={'Role:'}
          />{' '}
          {role === 'admin' ? 'Admin' : 'Developer'}
        </Typography> */}
        <ColorSchemeToggle />

        {/* <Tooltip title="Notifications">
          <IconButton
            onClick={shellActions.toggleNotificationPanel}
            size="small"
            sx={{ color: 'text.secondary' }}
          >
            <Badge
              badgeContent={unreadCount ?? 0}
              color="error"
              max={99}
              invisible={(unreadCount ?? 0) === 0}
            >
              <Bell size={20} />
            </Badge>
          </IconButton>
        </Tooltip> */}

        <Divider
          orientation="vertical"
          flexItem
          sx={{ mx: 1, display: { xs: 'none', sm: 'block' } }}
        />

        <UserMenu
          user={userForMenu as any}
          onLogout={onLogout}
        />

        {/* If you want, you can move confirm dialog OUT of header too.
            For now keeping same behavior by exposing state up is cleaner. */}
        {/* {confirmDialogOpen && (
          <Button
            sx={{ display: 'none' }}
            onClick={() => setConfirmDialogOpen(false)}
          />
        )} */}
      </Header.Actions>
    </Header>
  );
}
