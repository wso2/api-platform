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
  Grid,
} from '@wso2/oxygen-ui';
import { Boxes, Layers } from '@wso2/oxygen-ui-icons-react';
import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import {useOrganization} from '../../../../api/resources/organizations';
import { QuickStartBanner } from '../../../../components/cards/QuickStartBanner';
import {
  SummaryCardSection,
  type SummaryRow,
} from '../../../../components/cards/SummaryCardSection';
import { ErrorState, LoadingState } from '../../../../components/StateViews';
import { routes } from '../../../../routes/paths';
import { useConsoleScope } from '../../../../scope/ConsoleScopeProvider';
import { relativeTime } from '../../../../utils/relativeTime';
import { NewProjectDialog } from '../projects/NewProjectDialog';
import ExploreMoreCard from './ExploreMoreCard';

export function OrganizationHomePage() {
  const navigate = useNavigate();
  const { isLoading, organization, organizations, params, projects, projectsError } =
    useConsoleScope();
  const orgHandle = params.orgHandle || '';
  const [createOpen, setCreateOpen] = useState(false);
  const organizationQuery = useOrganization(orgHandle);
  const currentOrganization =
    organizationQuery.data || organization || organizations[0];

  const projectRows = useMemo<SummaryRow[]>(
    () =>
      [...projects]
        .sort(
          (left, right) =>
            new Date(right.updatedAt || 0).getTime() -
            new Date(left.updatedAt || 0).getTime()
        )
        .map((project) => ({
          id: project.id,
          title: project.displayName || project.id,
          description: project.description,
          meta: project.updatedAt ? relativeTime(project.updatedAt) : undefined,
        })),
    [projects]
  );

  if (isLoading && !currentOrganization) {
    return <LoadingState label="Loading organization" />;
  }
  if (!currentOrganization) {
    return (
      <ErrorState message="Unable to resolve organization context. The organization API returned no usable organizations for this session." />
    );
  }

  return (
    <>
      <Grid container spacing={3} sx={{ m: 0, width: '100%' }}>
        <Grid size={{ xs: 12 }}>
          <QuickStartBanner
            actionLabel="View projects"
            description={
              'Projects organize the APIs you build.'
            }
            icon={<Layers size={22} />}
            onAction={() => navigate(routes.projects(orgHandle))}
            title={`Welcome to ${currentOrganization.displayName}`}
          />
        </Grid>
          
        <SummaryCardSection
            addLabel="New project"
            emptyActionLabel="Create project"
            emptyDescription="Create a project to organize and manage your APIs."
            emptyTitle="Get started with a project"
            error={projectsError}
            icon={<Boxes size={24} />}
            items={projectRows}
            onAdd={() => setCreateOpen(true)}
            onEmptyAction={() => setCreateOpen(true)}
            onItemClick={(handler) =>
              navigate(routes.projectHome(orgHandle, handler))
            }
            onSeeMore={() => navigate(routes.projects(orgHandle))}
            title="Projects"
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
