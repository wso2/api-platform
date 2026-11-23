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

import { useMemo } from 'react';
import { defineMessages, useIntl } from 'react-intl';
import { Navigate, useParams } from 'react-router-dom';

import { useAllProjects } from '@/api/resources/projects';
import { AppPage } from '@/components/AppPage';
import { LoadingState } from '@/components/StateViews';
import { latestProjectId, readStoredScope } from '@/navigation/landing';
import { routes } from '@/routes/paths';

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.system.OrganizationLandingPage.loading',
    defaultMessage: 'Loading organization',
    description: "Shown while the organization's projects are read to choose where to land.",
  },
});

/**
 * Where a user first lands in an organization: a project's home.
 *
 * Runs inside the organization's own scope, so the session is already in it and
 * every one of its projects can be read, whichever organization the user came
 * from. The project the header last remembered, if it is still there; else the
 * one changed most recently; else, with no projects, the organization's home.
 */
export function OrganizationLandingPage() {
  return (
    <AppPage>
      <OrganizationLanding />
    </AppPage>
  );
}

function OrganizationLanding() {
  const intl = useIntl();
  const { orgHandle = '' } = useParams();
  const stored = useMemo(readStoredScope, []);
  const projectsQuery = useAllProjects();

  if (projectsQuery.isPending && !projectsQuery.error) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }

  const projects = projectsQuery.data ?? [];
  const remembered =
    stored?.organization === orgHandle && projects.some((item) => item.id === stored.project)
      ? stored.project
      : undefined;
  const projectId = remembered ?? latestProjectId(projects);

  return (
    <Navigate
      replace
      to={projectId ? routes.projectHome(orgHandle, projectId) : routes.organizationHome(orgHandle)}
    />
  );
}
