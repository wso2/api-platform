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

import { Grid } from '@wso2/oxygen-ui';
import { Boxes, Building } from '@wso2/oxygen-ui-icons-react';
import { useMemo, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';

import { useOrganization } from '@/api/resources/organizations';
import { QuickStartBanner } from '@/components/common/QuickStartBanner';
import { SummaryCardSection, type SummaryRow } from '@/components/common/SummaryCardSection';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { relativeTime } from '@/utils/relativeTime';
import { NewProjectDialog } from '@/pages/appShell/appShellPages/projects/components/NewProjectDialog';
import ExploreMoreCard from './components/ExploreMoreCard';

const messages = defineMessages({
  bannerAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.bannerAction',
    defaultMessage: 'View projects',
    description: 'Button on the welcome banner that opens the project listing.',
  },
  bannerDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.bannerDescription',
    defaultMessage: 'Projects organize the APIs you build.',
  },
  bannerTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.bannerTitle',
    defaultMessage: 'Welcome to {organizationName}',
    description: '{organizationName} is the display name of the organization. Never translated.',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.loading',
    defaultMessage: 'Loading organization',
  },
  projectsAdd: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectsAdd',
    defaultMessage: 'New project',
    description: 'Action that opens the create-project dialog from the section header.',
  },
  projectsEmptyAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectsEmptyAction',
    defaultMessage: 'Create project',
  },
  projectsEmptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectsEmptyDescription',
    defaultMessage: 'Create a project to organize and manage your APIs.',
  },
  projectsEmptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectsEmptyTitle',
    defaultMessage: 'Get started with a project',
  },
  projectsTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectsTitle',
    defaultMessage: 'Projects',
    description: 'Heading of the projects summary section. Noun, not a command.',
  },
  unresolvedOrganization: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.unresolvedOrganization',
    defaultMessage:
      'Unable to resolve organization context. The organization API returned no usable organizations for this session.',
  },
});

export function OrganizationHomePage() {
  const intl = useIntl();
  const navigate = useNavigate();
  const { isLoading, organization, organizations, params, projects, projectsError } =
    useConsoleScope();
  const orgHandle = params.orgHandle || '';
  const [createOpen, setCreateOpen] = useState(false);
  const organizationQuery = useOrganization(orgHandle);
  const currentOrganization = organizationQuery.data || organization || organizations[0];

  const projectRows = useMemo<SummaryRow[]>(
    () =>
      [...projects]
        .sort(
          (left, right) =>
            new Date(right.updatedAt || 0).getTime() - new Date(left.updatedAt || 0).getTime(),
        )
        .map((project) => ({
          id: project.id,
          title: project.displayName || project.id,
          description: project.description,
          meta: project.updatedAt ? relativeTime(project.updatedAt) : undefined,
        })),
    [projects],
  );

  if (isLoading && !currentOrganization) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (!currentOrganization) {
    return <ErrorState message={intl.formatMessage(messages.unresolvedOrganization)} />;
  }

  return (
    <>
      <Grid container spacing={3} sx={{ m: 0, width: '100%' }}>
        <Grid size={{ xs: 12 }}>
          <QuickStartBanner
            actionLabel={intl.formatMessage(messages.bannerAction)}
            description={intl.formatMessage(messages.bannerDescription)}
            icon={<Building size={32} />}
            onAction={() => navigate(routes.projects(orgHandle))}
            title={intl.formatMessage(messages.bannerTitle, {
              organizationName: currentOrganization.displayName,
            })}
          />
        </Grid>

        <SummaryCardSection
          addLabel={intl.formatMessage(messages.projectsAdd)}
          emptyActionLabel={intl.formatMessage(messages.projectsEmptyAction)}
          emptyDescription={intl.formatMessage(messages.projectsEmptyDescription)}
          emptyTitle={intl.formatMessage(messages.projectsEmptyTitle)}
          error={projectsError}
          icon={<Boxes size={24} />}
          items={projectRows}
          onAdd={() => setCreateOpen(true)}
          onEmptyAction={() => setCreateOpen(true)}
          onItemClick={(handler) => navigate(routes.projectHome(orgHandle, handler))}
          onSeeMore={() => navigate(routes.projects(orgHandle))}
          title={intl.formatMessage(messages.projectsTitle)}
          totalCount={projects.length}
        />

        <Grid size={{ xs: 12 }}>
          <ExploreMoreCard />
        </Grid>
      </Grid>

      <NewProjectDialog
        onClose={() => setCreateOpen(false)}
        open={createOpen}
        orgHandle={orgHandle}
      />
    </>
  );
}
