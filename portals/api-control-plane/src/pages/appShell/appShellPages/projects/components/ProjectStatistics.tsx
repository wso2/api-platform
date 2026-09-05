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

import { Box, ButtonBase, Divider, Grid, Skeleton, Stack, Typography } from '@wso2/oxygen-ui';
import { Braces, Boxes, Network, Radio } from '@wso2/oxygen-ui-icons-react';
import type { ReactNode } from 'react';
import { defineMessages, FormattedMessage, FormattedNumber, useIntl } from 'react-intl';

import { useAllRestApis } from '@/api/resources/restApis';
import {
  matchesApiType,
  type ApiTypeFilter,
} from '@/pages/appShell/appShellPages/apis/listing/apiTypeFilter';

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
  graphql: {
    id: 'apiControlPlane.projects.ProjectStatistics.graphql',
    defaultMessage: 'GraphQL',
  },
  grpc: { id: 'apiControlPlane.projects.ProjectStatistics.grpc', defaultMessage: 'gRPC' },
  rest: { id: 'apiControlPlane.projects.ProjectStatistics.rest', defaultMessage: 'REST' },
  selectType: {
    id: 'apiControlPlane.projects.ProjectStatistics.selectType',
    defaultMessage: 'Filter APIs by {type}',
  },
  statusSummary: {
    id: 'apiControlPlane.projects.ProjectStatistics.statusSummary',
    defaultMessage: '{published} published · {created} created',
  },
});

const ApiTypeMetric = ({
  ariaLabel,
  icon,
  label,
  onClick,
  selected,
  value,
}: {
  ariaLabel: string;
  icon: ReactNode;
  label: ReactNode;
  onClick: () => void;
  selected: boolean;
  value?: number;
}) => (
  <ButtonBase
    aria-label={ariaLabel}
    aria-pressed={selected}
    onClick={onClick}
    sx={{
      '&:hover': { bgcolor: 'action.hover' },
      bgcolor: selected ? 'action.selected' : 'transparent',
      border: 1,
      borderColor: selected ? 'primary.main' : 'transparent',
      borderRadius: 1,
      px: 1.5,
      py: 1,
      transition: 'background-color 150ms ease, border-color 150ms ease',
      width: '100%',
    }}
  >
    <Stack alignItems="center" direction="row" spacing={1.25} sx={{ width: '100%' }}>
      <Box sx={{ color: selected ? 'primary.main' : 'text.secondary', display: 'flex' }}>
        {icon}
      </Box>
      <Typography color={selected ? 'text.primary' : 'text.secondary'} variant="body1">
        {label}
      </Typography>
      {value === undefined ? (
        <Skeleton height={24} width={24} />
      ) : (
        <Typography sx={{ fontWeight: 700 }} variant="h6">
          <FormattedNumber value={value} />
        </Typography>
      )}
    </Stack>
  </ButtonBase>
);

type ProjectStatisticsProps = {
  onTypeFilterChange: (type: ApiTypeFilter | null) => void;
  selectedType: ApiTypeFilter | null;
};

export function ProjectStatistics({ onTypeFilterChange, selectedType }: ProjectStatisticsProps) {
  const intl = useIntl();
  const apisQuery = useAllRestApis();
  const total = apisQuery.data?.pagination.total;
  const apis = apisQuery.data?.list;
  const countType = (type: ApiTypeFilter) =>
    apis?.filter((api) => matchesApiType(api.kind, type)).length;
  const published = apis?.filter((api) => api.lifeCycleStatus === 'PUBLISHED').length;
  const created = apis?.filter((api) => api.lifeCycleStatus === 'CREATED').length;
  const selectType = (type: ApiTypeFilter) =>
    onTypeFilterChange(selectedType === type ? null : type);
  const filterLabel = (label: string) => intl.formatMessage(messages.selectType, { type: label });

  if (!apisQuery.isPending && total === 0) {
    return null;
  }

  return (
    <Grid alignItems="center" container sx={{ minHeight: 80, py: 1.5 }}>
      <Grid size={{ md: 2, xs: 12 }}>
        <Stack spacing={0.5}>
          <Typography color="text.secondary" sx={{ textTransform: 'uppercase' }} variant="caption">
            <FormattedMessage {...messages.apis} />
          </Typography>
          <Stack alignItems="baseline" direction="row" spacing={1}>
            {total === undefined ? (
              <Skeleton height={40} width={32} />
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
          </Stack>
        </Stack>
      </Grid>

      <Grid size={{ md: 'auto', xs: 12 }} sx={{ alignSelf: 'stretch', px: { md: 2.5, xs: 0 } }}>
        <Divider orientation="vertical" sx={{ display: { md: 'block', xs: 'none' } }} />
        <Divider sx={{ display: { md: 'none', xs: 'block' } }} />
      </Grid>

      <Grid size={{ md: 7, xs: 12 }}>
        <Grid container spacing={2.5}>
          <Grid size={{ lg: 3, sm: 6, xs: 12 }}>
            <ApiTypeMetric
              ariaLabel={filterLabel(intl.formatMessage(messages.rest))}
              icon={<Boxes size={16} />}
              label={<FormattedMessage {...messages.rest} />}
              onClick={() => selectType('rest')}
              selected={selectedType === 'rest'}
              value={countType('rest')}
            />
          </Grid>
          <Grid size={{ lg: 3, sm: 6, xs: 12 }}>
            <ApiTypeMetric
              ariaLabel={filterLabel(intl.formatMessage(messages.graphql))}
              icon={<Braces size={16} />}
              label={<FormattedMessage {...messages.graphql} />}
              onClick={() => selectType('graphql')}
              selected={selectedType === 'graphql'}
              value={countType('graphql')}
            />
          </Grid>
          <Grid size={{ lg: 3, sm: 6, xs: 12 }}>
            <ApiTypeMetric
              ariaLabel={filterLabel(intl.formatMessage(messages.async))}
              icon={<Radio size={16} />}
              label={<FormattedMessage {...messages.async} />}
              onClick={() => selectType('async')}
              selected={selectedType === 'async'}
              value={countType('async')}
            />
          </Grid>
          <Grid size={{ lg: 3, sm: 6, xs: 12 }}>
            <ApiTypeMetric
              ariaLabel={filterLabel(intl.formatMessage(messages.grpc))}
              icon={<Network size={16} />}
              label={<FormattedMessage {...messages.grpc} />}
              onClick={() => selectType('grpc')}
              selected={selectedType === 'grpc'}
              value={countType('grpc')}
            />
          </Grid>
        </Grid>
      </Grid>

      <Grid size={{ md: 'auto', xs: 12 }} sx={{ alignSelf: 'stretch', px: { md: 2.5, xs: 0 } }}>
        <Divider orientation="vertical" sx={{ display: { md: 'block', xs: 'none' } }} />
        <Divider sx={{ display: { md: 'none', xs: 'block' } }} />
      </Grid>

      <Grid size={{ md: 2, xs: 12 }}>
        <Stack alignItems="center" direction="row" spacing={1.25}>
          <Network size={16} />
          <Typography color="text.secondary" variant="body1">
            <FormattedMessage {...messages.deployments} />
          </Typography>
          <Box sx={{ ml: 'auto' }}>
            <Skeleton height={24} width={28} />
          </Box>
        </Stack>
      </Grid>
    </Grid>
  );
}
