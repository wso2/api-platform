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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { Card, CardContent, Divider, Grid, Skeleton, Stack, Typography } from '@wso2/oxygen-ui';
import { Braces, Boxes, Network, Radio } from '@wso2/oxygen-ui-icons-react';
import type { ReactNode } from 'react';
import { defineMessages, FormattedMessage, FormattedNumber } from 'react-intl';

import { useRestApis } from '@/api/resources/restApis';

const messages = defineMessages({
  apis: { id: 'apiControlPlane.projects.ProjectStatistics.apis', defaultMessage: 'APIs' },
  async: {
    id: 'apiControlPlane.projects.ProjectStatistics.async',
    defaultMessage: 'Async / events',
  },
  deployments: {
    id: 'apiControlPlane.projects.ProjectStatistics.deployments',
    defaultMessage: 'Deployments',
  },
  environments: {
    id: 'apiControlPlane.projects.ProjectStatistics.environments',
    defaultMessage: 'across environments',
  },
  errorRate: {
    id: 'apiControlPlane.projects.ProjectStatistics.errorRate',
    defaultMessage: 'Error rate',
  },
  graphql: {
    id: 'apiControlPlane.projects.ProjectStatistics.graphql',
    defaultMessage: 'GraphQL',
  },
  grpc: { id: 'apiControlPlane.projects.ProjectStatistics.grpc', defaultMessage: 'gRPC' },
  requests: {
    id: 'apiControlPlane.projects.ProjectStatistics.requests',
    defaultMessage: 'Requests (24h)',
  },
  rest: { id: 'apiControlPlane.projects.ProjectStatistics.rest', defaultMessage: 'REST' },
  statusSummary: {
    id: 'apiControlPlane.projects.ProjectStatistics.statusSummary',
    defaultMessage: '{published} published · {created} created',
  },
});

const ApiTypeMetric = ({
  icon,
  label,
  value,
}: {
  icon: ReactNode;
  label: ReactNode;
  value?: number;
}) => (
  <Stack alignItems="center" direction="row" justifyContent="space-between" spacing={1}>
    <Stack alignItems="center" direction="row" spacing={0.75}>
      {icon}
      <Typography color="text.secondary" variant="body2">
        {label}
      </Typography>
    </Stack>
    {value === undefined ? (
      <Skeleton height={24} width={24} />
    ) : (
      <Typography sx={{ fontWeight: 600 }} variant="body2">
        <FormattedNumber value={value} />
      </Typography>
    )}
  </Stack>
);

const PendingMetricCard = ({ label }: { label: ReactNode }) => (
  <Card sx={{ height: '100%' }} variant="outlined">
    <CardContent
      sx={{
        '&:last-child': { pb: 2 },
        alignItems: 'center',
        display: 'flex',
        height: '100%',
        p: 2,
      }}
    >
      <Stack spacing={1.5}>
        <Typography color="text.secondary" sx={{ textTransform: 'uppercase' }} variant="caption">
          {label}
        </Typography>
        <Skeleton height={48} width="45%" />
        <Skeleton height={20} width="65%" />
      </Stack>
    </CardContent>
  </Card>
);

export function ProjectStatistics() {
  const apisQuery = useRestApis({ limit: 100, offset: 0 });
  const total = apisQuery.data?.pagination.total;
  const apis = apisQuery.data?.list;
  const countKind = (...kinds: string[]) =>
    apis?.filter((api) => kinds.includes(api.kind?.toLowerCase() ?? '')).length;
  const published = apis?.filter((api) => api.lifeCycleStatus === 'PUBLISHED').length;
  const created = apis?.filter((api) => api.lifeCycleStatus === 'CREATED').length;

  return (
    <Grid container spacing={2}>
      <Grid size={{ lg: 6, xs: 12 }}>
        <Card sx={{ height: '100%' }} variant="outlined">
          <CardContent
            sx={{
              '&:last-child': { pb: 2 },
              alignItems: 'center',
              display: 'flex',
              height: '100%',
              p: 2,
            }}
          >
            <Grid container spacing={2} sx={{ width: '100%' }}>
              <Grid size={{ sm: 3, xs: 12 }}>
                <Typography
                  color="text.secondary"
                  sx={{ textTransform: 'uppercase' }}
                  variant="caption"
                >
                  <FormattedMessage {...messages.apis} />
                </Typography>
                {total === undefined ? (
                  <Skeleton height={52} width={64} />
                ) : (
                  <Typography sx={{ fontWeight: 700 }} variant="h2">
                    <FormattedNumber value={total} />
                  </Typography>
                )}
                {published === undefined || created === undefined ? (
                  <Skeleton height={20} width={112} />
                ) : (
                  <Typography color="text.secondary" variant="caption">
                    <FormattedMessage {...messages.statusSummary} values={{ created, published }} />
                  </Typography>
                )}
              </Grid>
              <Grid size={{ sm: 9, xs: 12 }}>
                <Grid container spacing={1.25}>
                  <Grid size={{ sm: 6, xs: 12 }}>
                    <ApiTypeMetric
                      icon={<Boxes size={16} />}
                      label={<FormattedMessage {...messages.rest} />}
                      value={total}
                    />
                  </Grid>
                  <Grid size={{ sm: 6, xs: 12 }}>
                    <ApiTypeMetric
                      icon={<Braces size={16} />}
                      label={<FormattedMessage {...messages.graphql} />}
                      value={countKind('graphql')}
                    />
                  </Grid>
                  <Grid size={{ sm: 6, xs: 12 }}>
                    <ApiTypeMetric
                      icon={<Radio size={16} />}
                      label={<FormattedMessage {...messages.async} />}
                      value={countKind('async', 'asyncapi', 'event')}
                    />
                  </Grid>
                  <Grid size={{ sm: 6, xs: 12 }}>
                    <ApiTypeMetric
                      icon={<Network size={16} />}
                      label={<FormattedMessage {...messages.grpc} />}
                      value={countKind('grpc')}
                    />
                  </Grid>
                </Grid>
                <Divider sx={{ my: 1 }} />
                <Stack alignItems="center" direction="row" justifyContent="space-between">
                  <Stack alignItems="center" direction="row" spacing={0.75}>
                    <Network size={16} />
                    <Typography color="text.secondary" variant="body2">
                      <FormattedMessage {...messages.deployments} />
                    </Typography>
                  </Stack>
                  <Stack alignItems="center" direction="row" spacing={1}>
                    <Skeleton height={24} width={28} />
                    <Typography color="text.secondary" variant="caption">
                      <FormattedMessage {...messages.environments} />
                    </Typography>
                  </Stack>
                </Stack>
              </Grid>
            </Grid>
          </CardContent>
        </Card>
      </Grid>
      <Grid size={{ lg: 3, sm: 6, xs: 12 }}>
        <PendingMetricCard label={<FormattedMessage {...messages.requests} />} />
      </Grid>
      <Grid size={{ lg: 3, sm: 6, xs: 12 }}>
        <PendingMetricCard label={<FormattedMessage {...messages.errorRate} />} />
      </Grid>
    </Grid>
  );
}
