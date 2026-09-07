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

import type { ReactNode } from 'react';
import { Box, ButtonBase, Divider, Grid, Skeleton, Stack, Typography } from '@wso2/oxygen-ui';
import { Network } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, FormattedNumber, useIntl } from 'react-intl';

import { useAllRestApis } from '@/api/resources/restApis';
import grpcIcon from '@/assets/icons/gRPC.svg';
import graphqlIcon from '@/assets/icons/graphql.svg';
import restIcon from '@/assets/icons/rest.svg';
import websocketIcon from '@/assets/icons/websocket.svg';
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
  graphql: { id: 'apiControlPlane.projects.ProjectStatistics.graphql', defaultMessage: 'GraphQL' },
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

function MetricCard({
  ariaLabel,
  iconSrc,
  label,
  onClick,
  selected = false,
  value,
}: {
  ariaLabel: string;
  iconSrc: string;
  label: ReactNode;
  onClick?: () => void;
  selected?: boolean;
  value?: number;
}) {
  return (
    <ButtonBase
      aria-label={ariaLabel}
      aria-pressed={onClick ? selected : undefined}
      disabled={!onClick}
      onClick={onClick}
      sx={{
        '&:hover': onClick ? { bgcolor: 'action.hover' } : undefined,
        bgcolor: selected ? 'action.selected' : 'transparent',
        border: 1,
        borderColor: selected ? 'primary.main' : 'transparent',
        borderRadius: 1,
        px: 1.25,
        py: 1,
        transition: 'background-color 150ms ease, border-color 150ms ease',
        width: '100%',
      }}
    >
      <Stack alignItems="center" direction="row" spacing={1.25} sx={{ width: '100%' }}>
        <Box
          sx={{
            alignItems: 'center',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 0.4,
            display: 'flex',
            flexShrink: 0,
            height: 26,
            justifyContent: 'center',
            width: 26,
          }}
        >
          <Box
            alt=""
            aria-hidden
            component="img"
            src={iconSrc}
            sx={{ height: 22, objectFit: 'contain', width: 22 }}
          />
        </Box>
        <Box sx={{ minWidth: 0, textAlign: 'left' }}>
          <Typography color={selected ? 'text.primary' : 'text.secondary'} noWrap variant="body1">
            {label}
          </Typography>
          {value === undefined ? (
            <Skeleton height={30} width={30} />
          ) : (
            <Typography sx={{ fontWeight: 700, lineHeight: 1.15 }} variant="h6">
              <FormattedNumber value={value} />
            </Typography>
          )}
        </Box>
      </Stack>
    </ButtonBase>
  );
}

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

  if (!apisQuery.isPending && total === 0) return null;

  const metrics = [
    { type: 'rest' as const, message: messages.rest, icon: restIcon },
    { type: 'graphql' as const, message: messages.graphql, icon: graphqlIcon },
    { type: 'async' as const, message: messages.async, icon: websocketIcon },
    { type: 'grpc' as const, message: messages.grpc, icon: grpcIcon },
  ];

  return (
    <Grid alignItems="center" container sx={{ minHeight: 68, py: 1 }}>
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
          {metrics.map(({ type, message, icon }) => {
            const label = intl.formatMessage(message);
            return (
              <Grid key={type} size={{ md: 3, sm: 6, xs: 12 }}>
                <MetricCard
                  ariaLabel={filterLabel(label)}
                  iconSrc={icon}
                  label={label}
                  onClick={() => selectType(type)}
                  selected={selectedType === type}
                  value={countType(type)}
                />
              </Grid>
            );
          })}
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
