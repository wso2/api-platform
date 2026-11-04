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
import {
  Box,
  ButtonBase,
  Card,
  CardHeader,
  CardContent,
  CardActions,
  Divider,
  Skeleton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Braces, ChartColumn } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, FormattedNumber, useIntl } from 'react-intl';

import { useAllGraphQLApis } from '@/api/resources/graphqlApis';
import { useAllRestApis } from '@/api/resources/restApis';
import grpcIcon from '@/assets/icons/gRPC.svg';
import graphqlIcon from '@/assets/icons/graphql.svg';
import websocketIcon from '@/assets/icons/websocket.svg';
import {
  matchesApiType,
  type ApiTypeFilter,
} from '@/pages/appShell/appShellPages/apis/listing/apiTypeFilter';

const messages = defineMessages({
  apis: {
    id: 'apiControlPlane.projects.ProjectStatistics.apis',
    defaultMessage: 'API types',
  },
  async: {
    id: 'apiControlPlane.projects.ProjectStatistics.async',
    defaultMessage: 'Async / events',
  },
  total: {
    id: 'apiControlPlane.projects.ProjectStatistics.total',
    defaultMessage: 'Total',
  },
  graphql: { id: 'apiControlPlane.projects.ProjectStatistics.graphql', defaultMessage: 'GraphQL' },
  grpc: { id: 'apiControlPlane.projects.ProjectStatistics.grpc', defaultMessage: 'gRPC' },
  rest: { id: 'apiControlPlane.projects.ProjectStatistics.rest', defaultMessage: 'REST' },
  selectType: {
    id: 'apiControlPlane.projects.ProjectStatistics.selectType',
    defaultMessage: 'Filter APIs by {type}',
  },
});

function MetricCard({
  ariaLabel,
  icon,
  label,
  onClick,
  selected = false,
  value,
}: {
  ariaLabel: string;
  icon: ReactNode;
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
        borderRadius: '6px',
        '&.Mui-focusVisible': {
          outline: '2px solid',
          outlineColor: 'primary.main',
          outlineOffset: 2,
        },
        px: 1.25,
        py: 1.1,
        transition: 'background-color 150ms ease, border-color 150ms ease',
        width: '100%',
      }}
    >
      <Stack alignItems="center" direction="row" spacing={1.25} sx={{ width: '100%' }}>
        {icon}
        <Typography color="text.secondary" sx={{ flex: 1, textAlign: 'left' }} variant="body2">
          {label}
        </Typography>
        {value === undefined ? (
          <Skeleton width={24} />
        ) : (
          <Typography sx={{ fontWeight: 700 }} variant="body2">
            <FormattedNumber value={value} />
          </Typography>
        )}
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
  // Same two sources `ApiList` merges — the counts must agree with the list below.
  const restApisQuery = useAllRestApis();
  const graphqlApisQuery = useAllGraphQLApis();
  const isPending = restApisQuery.isPending || graphqlApisQuery.isPending;
  const restApis = restApisQuery.data?.list;
  const graphqlApis = graphqlApisQuery.data?.list;
  const total =
    restApisQuery.data && graphqlApisQuery.data
      ? restApisQuery.data.pagination.total + graphqlApisQuery.data.pagination.total
      : undefined;
  // GraphQL is counted by source, not `kind`, matching `ApiList`'s filter.
  const countType = (type: ApiTypeFilter) =>
    type === 'graphql'
      ? graphqlApis?.length
      : restApis?.filter((api) => matchesApiType(api.kind, type)).length;
  const selectType = (type: ApiTypeFilter) =>
    onTypeFilterChange(selectedType === type ? null : type);
  const filterLabel = (label: string) => intl.formatMessage(messages.selectType, { type: label });

  if (!isPending && total === 0) return null;

  const typeIcon = (type: ApiTypeFilter, src?: string) => (
    <Box
      aria-hidden
      sx={{
        width: 28,
        height: 28,
        flexShrink: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        border: 1,
        borderColor: 'divider',
        borderRadius: '6px',
        bgcolor: type === 'grpc' ? '#254c4c' : 'action.hover',
        color: '#A6B3FF',
        overflow: 'hidden',
      }}
    >
      {type === 'rest' ? (
        <Braces size={16} strokeWidth={2} />
      ) : (
        <Box
          component="img"
          src={src}
          alt=""
          sx={{
            width: type === 'grpc' ? 28 : 16,
            height: type === 'grpc' ? 28 : 16,
            objectFit: 'contain',
          }}
        />
      )}
    </Box>
  );
  const metrics = [
    { type: 'rest' as const, message: messages.rest, icon: typeIcon('rest') },
    { type: 'graphql' as const, message: messages.graphql, icon: typeIcon('graphql', graphqlIcon) },
    { type: 'async' as const, message: messages.async, icon: typeIcon('async', websocketIcon) },
    { type: 'grpc' as const, message: messages.grpc, icon: typeIcon('grpc', grpcIcon) },
  ];

  return (
    <Card variant="outlined" sx={{ borderRadius: '14px' }}>
      <CardHeader
        avatar={<ChartColumn size={18} strokeWidth={1.5} />}
        title={intl.formatMessage(messages.apis)}
        slotProps={{
          title: { variant: 'subtitle2', color: 'text.secondary', sx: { fontWeight: 600 } },
        }}
        sx={{ px: 2.25, pt: 2.25, pb: 1 }}
      />
      <CardContent sx={{ px: 2.25, py: 0 }}>
        <Stack divider={<Divider />}>
          {metrics.map(({ type, message, icon }) => {
            const label = intl.formatMessage(message);
            return (
              <MetricCard
                key={type}
                ariaLabel={filterLabel(label)}
                icon={icon}
                label={label}
                onClick={() => selectType(type)}
                selected={selectedType === type}
                value={countType(type)}
              />
            );
          })}
        </Stack>
        <Divider />
      </CardContent>
      <CardActions sx={{ px: 2.25, pt: 1.5, pb: 2.25 }}>
        <ButtonBase
          onClick={() => onTypeFilterChange(null)}
          aria-pressed={selectedType === null}
          sx={{
            width: '100%',
            display: 'flex',
            justifyContent: 'space-between',
            p: 1.5,
            bgcolor: 'action.hover',
            borderRadius: '8px',
            border: 1,
            borderColor: 'divider',
            '&:hover': { bgcolor: 'action.selected' },
            '&.Mui-focusVisible': {
              outline: '2px solid',
              outlineColor: 'primary.main',
              outlineOffset: 2,
            },
          }}
        >
          <Typography variant="body2" sx={{ fontWeight: 700 }}>
            <FormattedMessage {...messages.total} />
          </Typography>
          {total === undefined ? (
            <Skeleton width={24} />
          ) : (
            <Typography color="primary" variant="body2" sx={{ fontWeight: 700 }}>
              <FormattedNumber value={total} />
            </Typography>
          )}
        </ButtonBase>
      </CardActions>
    </Card>
  );
}
