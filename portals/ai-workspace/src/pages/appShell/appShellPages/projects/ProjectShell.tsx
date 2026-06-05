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

import React, { useEffect, useMemo } from 'react';
import { Outlet, useNavigate, useParams } from 'react-router-dom';
import { PageContent, Stack, Typography } from '@wso2/oxygen-ui';
import { useAppShell } from '../../../../contexts/AppShellContext';
import AILoader from '../../../../Components/AILoader';
import { buildOrgPath, getProjectSlug } from '../../../../utils/projectRouting';
import { FormattedMessage } from 'react-intl';

export default function ProjectShell() {
  const navigate = useNavigate();
  const { projectSlug } = useParams<{ projectSlug: string }>();
  const {
    projectsForCurrentOrganization,
    setCurrentProject,
    currentProject,
    isProjectsLoading,
    currentOrganization,
  } = useAppShell();

  const matchedProject = useMemo(() => {
    if (!projectSlug) return null;
    return (
      projectsForCurrentOrganization.find(
        (project) => getProjectSlug(project) === projectSlug
      ) ?? null
    );
  }, [projectSlug, projectsForCurrentOrganization]);

  useEffect(() => {
    if (!matchedProject) return;
    if (currentProject?.id !== matchedProject.id) {
      setCurrentProject(matchedProject);
    }
  }, [matchedProject, currentProject, setCurrentProject]);

  if (isProjectsLoading) {
    return (
      <PageContent fullWidth>
        <Stack spacing={2} alignItems="center" sx={{ py: 6 }}>
          <AILoader />
          <Typography variant="body2" color="text.secondary">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectShell.loading.project"
              defaultMessage={'Loading project...'}
            />
          </Typography>
        </Stack>
      </PageContent>
    );
  }

  if (!matchedProject) {
    return (
      <PageContent fullWidth>
        <Stack spacing={1.5} alignItems="flex-start">
          <Typography variant="h6">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectShell.project.not.found"
              defaultMessage={'Project not found'}
            />
          </Typography>
          <Typography variant="body2" color="text.secondary">
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectShell.the.selected.project.is.unavailable.please.choose.another.project"
              defaultMessage={
                'The selected project is unavailable. Please choose another project.'
              }
            />
          </Typography>
          <Typography
            variant="body2"
            color="primary"
            sx={{ cursor: 'pointer' }}
            onClick={() =>
              navigate(buildOrgPath(currentOrganization, '/projects/list'))
            }
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.projects.ProjectShell.go.to.projects"
              defaultMessage={'Go to projects'}
            />
          </Typography>
        </Stack>
      </PageContent>
    );
  }

  return <Outlet />;
}
