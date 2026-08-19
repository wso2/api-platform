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
  Chip,
  Grid,
  PageContent,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Boxes, Rocket } from '@wso2/oxygen-ui-icons-react';
import { useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';

import { useApis, useProject } from '../../../../api/hooks/useMvpQueries';
import { groupApisByKind } from '../../../../components/cards/apiDisplay';
import { QuickStartBanner } from '../../../../components/cards/QuickStartBanner';
import {
  SummaryCardSection,
  type SummaryRow,
} from '../../../../components/cards/SummaryCardSection';
import { ErrorState, LoadingState } from '../../../../components/StateViews';
import { routes } from '../../../../routes/paths';
import type { Api } from '../../../../types/domain';
import { relativeTime } from '../../../../utils/relativeTime';

export function ProjectHomePage() {
  const { orgHandle = '', projectHandler = '' } = useParams();
  const navigate = useNavigate();
  const projectQuery = useProject();
  const apisQuery = useApis();
  const components = apisQuery.data || [];
  const groups = groupApisByKind(components);

  const toRows = (list: Api[]): SummaryRow[] =>
    list.map((component) => ({
      id: component.handler,
      title: component.displayName,
      description: component.version
        ? `Version ${component.version}`
        : component.description,
      meta: component.createdAt
        ? relativeTime(component.createdAt)
        : component.updatedAt
          ? relativeTime(component.updatedAt)
          : undefined,
    }));

  const apiProxyRows = useMemo(() => toRows(groups.apiProxies), [groups.apiProxies]);

  const openApi = (handler: string) =>
    navigate(routes.api(orgHandle, projectHandler, handler));
  const onCreateApi = () =>
    navigate(routes.newApi(orgHandle, projectHandler));

  if (projectQuery.isLoading) return <LoadingState label="Loading project" />;
  if (projectQuery.error || !projectQuery.data) {
    return <ErrorState title="Project not found" />;
  }
  const project = projectQuery.data;

  return (
    <PageContent fullWidth>
      <Grid container spacing={3} sx={{ m: 0, width: '100%' }}>
        <Grid size={{ xs: 12 }}>
          <QuickStartBanner
            actionLabel="Create API"
            description={
              project.description ||
              'Add an API to this project.'
            }
            icon={<Boxes size={22} />}
            onAction={onCreateApi}
            title={project.name}
          />
        </Grid>

        <Grid size={{ xs: 12 }}>
          <SummaryCardSection
            addLabel="New"
            emptyActionLabel="Create API Proxy"
            emptyDescription="Expose and govern an API by creating an API Proxy."
            emptyTitle="Create your first API Proxy"
            error={apisQuery.error}
            icon={<Rocket size={24} />}
            isLoading={apisQuery.isLoading}
            items={apiProxyRows}
            onAdd={onCreateApi}
            onEmptyAction={onCreateApi}
            onItemClick={openApi}
            onSeeMore={() => navigate(routes.apis(orgHandle, projectHandler))}
            title="API Proxies"
            totalCount={groups.apiProxies.length}
          />
        </Grid>

        <Grid size={{ xs: 12 }}>
          <Card sx={{ width: '100%' }}>
            <CardContent>
              <Typography sx={{ fontWeight: 700 }} variant="h6">
                Project details
              </Typography>
              <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1, mt: 2 }}>
                <Chip label={`@${project.handler}`} size="small" variant="outlined" />
                {project.region && <Chip label={project.region} size="small" />}
                {project.version && (
                  <Chip label={`v${project.version}`} size="small" variant="outlined" />
                )}
                {project.type && (
                  <Chip
                    label={project.type === 'MONO_REPO' ? 'Mono repo' : 'Multi repo'}
                    size="small"
                    variant="outlined"
                  />
                )}
                {project.repository && (
                  <Chip label={project.repository} size="small" variant="outlined" />
                )}
                {project.createdDate && (
                  <Chip
                    label={`Created ${relativeTime(project.createdDate)}`}
                    size="small"
                    variant="outlined"
                  />
                )}
              </Stack>
              {!project.region &&
                !project.version &&
                !project.repository &&
                !project.createdDate && (
                  <Typography color="text.secondary" sx={{ mt: 1 }} variant="body2">
                    No additional project metadata available.
                  </Typography>
                )}
              <Box sx={{ mt: 2 }}>
                <Typography
                  color="primary"
                  onClick={() =>
                    navigate(routes.apis(orgHandle, projectHandler))
                  }
                  sx={{ cursor: 'pointer', display: 'inline-block', fontWeight: 600 }}
                  variant="body2"
                >
                  View all APIs ({components.length})
                </Typography>
              </Box>
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    </PageContent>
  );
}
