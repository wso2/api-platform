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
  Box,
  Card,
  CardContent,
  Grid,
  PageContent,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Boxes, Layers } from '@wso2/oxygen-ui-icons-react';
import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { useOrganization } from '../../api/hooks/useMvpQueries';
import { QuickStartBanner } from '../../components/cards/QuickStartBanner';
import {
  SummaryCardSection,
  type SummaryRow,
} from '../../components/cards/SummaryCardSection';
import { ErrorState, LoadingState } from '../../components/StateViews';
import { routes } from '../../routes/paths';
import { useConsoleScope } from '../../scope/ConsoleScopeProvider';
import { relativeTime } from '../../utils/relativeTime';
import { NewProjectDialog } from '../projects/NewProjectDialog';

const GETTING_STARTED = [
  {
    title: '1. Open a project',
    description: 'Projects group the APIs you build and operate.',
  },
  {
    title: '2. Create APIs',
    description: 'Add APIs inside a project.',
  },
  {
    title: '3. Deploy, test & observe',
    description: 'Use deploy, test, manage, and runtime logs per API.',
  },
];

export function OrganizationHomePage() {
  const navigate = useNavigate();
  const { isLoading, organization, organizations, params, projects, projectsError } =
    useConsoleScope();
  const orgHandle = params.orgHandle || '';
  const [createOpen, setCreateOpen] = useState(false);
  const organizationQuery = useOrganization();
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
          id: project.handler,
          title: project.name,
          description: project.region
            ? `@${project.handler} · ${project.region}`
            : `@${project.handler}`,
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
    <PageContent fullWidth>
      <Grid container spacing={3} sx={{ m: 0, width: '100%' }}>
        <Grid size={{ xs: 12 }}>
          <QuickStartBanner
            actionLabel="View projects"
            description={
              currentOrganization.description ||
              'Projects organize the APIs you build.'
            }
            icon={<Layers size={22} />}
            onAction={() => navigate(routes.projects(orgHandle))}
            title={`Welcome to ${currentOrganization.name}`}
          />
        </Grid>

        <Grid size={{ xs: 12, md: 7 }}>
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
        </Grid>

        <Grid size={{ xs: 12, md: 5 }}>
          <Card sx={{ height: '100%', minHeight: 300 }}>
            <CardContent>
              <Typography sx={{ fontWeight: 700 }} variant="h6">
                Getting started
              </Typography>
              <Stack spacing={2} sx={{ mt: 2 }}>
                {GETTING_STARTED.map((step) => (
                  <Box key={step.title}>
                    <Typography sx={{ fontWeight: 600 }}>{step.title}</Typography>
                    <Typography color="text.secondary" variant="body2">
                      {step.description}
                    </Typography>
                  </Box>
                ))}
              </Stack>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      <NewProjectDialog
        onClose={() => setCreateOpen(false)}
        open={createOpen}
        orgHandle={orgHandle}
      />
    </PageContent>
  );
}
