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

import React, { useMemo } from 'react';
import {
  ColorSchemeToggle,
  ComplexSelect,
  Divider,
  Header,
  IconButton,
  Box,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Building2, ChevronRight, X } from '@wso2/oxygen-ui-icons-react';
import SearchableComplexSelect from '../../Components/common/SearchableComplexSelect';
import Logo from '../../Components/Logo';
import UserMenu from '../../Components/UserMenu';
import { useRole } from '../../contexts/RoleContext';
import {
  buildOrgPath,
  buildProjectPath,
  getProjectSlug,
} from '../../utils/projectRouting';
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
};

type Props = {
  shellState: any;
  shellActions: any;
  navigate: (to: string) => void;

  userName?: string;
  userEmail?: string;

  currentOrganization: SelectableOrg | null;

  projectOptions: SelectableProject[];
  currentProject: SelectableProject | null;
  setCurrentProject?: (p: SelectableProject | null) => void;

  isProjectsLoading: boolean;
  projectsError?: unknown;

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

    currentOrganization,

    projectOptions,
    currentProject,
    setCurrentProject,
    isProjectsLoading,
    projectsError,

    selectedProjectId,
    setSelectedProjectId,

    onLogout,
  } = props;

  const userForMenu = useMemo(
    () => ({
      name: userName || userEmail || 'User',
      email: userEmail || '',
      role: role || undefined,
    }),
    [userName, userEmail, role]
  );

  const canShowProjectSwitcher = Boolean(currentProject?.id);
  const isProjectPickerDisabled = isProjectsLoading || !currentOrganization?.id;

  const handleProjectSelection = (nextProjectId: string) => {
    setSelectedProjectId(nextProjectId);
    const nextProject = projectOptions.find((p) => p.id === nextProjectId) ?? null;
    setCurrentProject?.(nextProject);
    if (nextProject?.id && currentOrganization?.id) {
      navigate(buildOrgPath(currentOrganization, `/projects/${getProjectSlug(nextProject)}/home`));
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
        <Stack direction="row" alignItems="center" spacing={0.5}>
          {currentOrganization && (
            <>
              <Box
                sx={{
                  display: 'inline-flex',
                  flexDirection: 'column',
                  justifyContent: 'center',
                  px: 1.25,
                  py: 0.5,
                  minWidth: 100,
                  height: 40,
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 1,
                  flexShrink: 0,
                  cursor: 'default',
                  userSelect: 'none',
                }}
              >
                <Typography
                  variant="caption"
                  sx={{ color: 'text.secondary', lineHeight: 1, mb: 0.25, fontSize: '0.65rem' }}
                >
                  Organizations
                </Typography>
                <Stack direction="row" alignItems="center" spacing={0.5}>
                  <Building2 size={13} />
                  <Typography
                    variant="body2"
                    fontWeight={500}
                    sx={{ color: 'text.primary', lineHeight: 1, fontSize: '0.8rem' }}
                  >
                    {currentOrganization.name}
                  </Typography>
                </Stack>
              </Box>
              <ChevronRight size={14} style={{ opacity: 0.4, flexShrink: 0 }} />
            </>
          )}

          {canShowProjectSwitcher ? (
            <Box sx={{ position: 'relative', minWidth: 220 }}>
              <SearchableComplexSelect
                value={selectedProjectId}
                selectedOption={currentProject}
                options={projectOptions}
                loading={isProjectsLoading}
                disabled={isProjectsLoading || !currentOrganization?.id}
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
              isProjectsLoading={isProjectsLoading}
              projectsError={projectsError}
              projectOptions={projectOptions}
              onSelectProject={handleProjectSelection}
            />
          )}
        </Stack>
      </Header.Switchers>

      <Header.Spacer />

      <Header.Actions>
        <ColorSchemeToggle />

        <Divider
          orientation="vertical"
          flexItem
          sx={{ mx: 1, display: { xs: 'none', sm: 'block' } }}
        />

        <UserMenu
          user={userForMenu as any}
          onLogout={onLogout}
        />
      </Header.Actions>
    </Header>
  );
}
