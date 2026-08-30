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

import { useMemo, useState } from 'react';
import {
  Box,
  Button,
  Chip,
  Grid,
  PageTitle,
  SearchBar,
  Stack,
  StatCard,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Layers,
  LayoutGrid,
  List,
  Network,
  Plus,
  Shrub,
  Wifi,
  WifiOff,
} from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate, useParams } from 'react-router-dom';

import { useGateways, type Gateway } from '@/api/resources/gateways';
import { GatewayIllustration } from '@/components/illustrations/GatewayIllustration';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { GatewayGridView } from './components/GatewayGridView';
import { GatewayListView } from './components/GatewayListView';
import { gatewayMode, gatewaySearchFields } from './utils/gatewayDisplay';
import { groupGatewaysByEnvironment } from './utils/gatewayEnvironments';

/** Which hosting modes the listing is currently showing. */
type GatewayFilter = 'all' | 'managed' | 'self';

/** How each environment group renders its gateways. */
type ViewMode = 'grid' | 'list';

const messages = defineMessages({
  emptyAction: {
    id: 'gateways.empty.action',
    defaultMessage: 'Provision gateway',
  },
  emptyDescription: {
    id: 'gateways.empty.description',
    defaultMessage: 'Provision a self-hosted gateway to start exposing APIs to clients.',
  },
  emptyTitle: {
    id: 'gateways.empty.title',
    defaultMessage: 'Provision your first gateway',
    description: 'First-run prompt shown to an organization with no gateways.',
  },
  errorMessage: {
    id: 'gateways.error.message',
    defaultMessage: 'Unable to load gateways',
  },
  filterAll: {
    id: 'gateways.filter.all',
    defaultMessage: 'All',
  },
  filterManaged: {
    id: 'gateways.filter.managed',
    defaultMessage: 'Managed',
  },
  filterSelfHosted: {
    id: 'gateways.filter.self',
    defaultMessage: 'Self-hosted',
  },
  gatewayCount: {
    id: 'gateways.count',
    defaultMessage: '{count, plural, one {# gateway} other {# gateways}}',
    description: 'Heading above the listing, counting every match.',
  },
  gridView: {
    id: 'gateways.view.grid',
    defaultMessage: 'Grid view',
    description: 'Accessible label for the button switching to the card grid.',
  },
  listView: {
    id: 'gateways.view.list',
    defaultMessage: 'List view',
    description: 'Accessible label for the button switching to compact rows.',
  },
  loading: {
    id: 'gateways.loading',
    defaultMessage: 'Loading gateways',
  },
  noMatchesDescription: {
    id: 'gateways.noMatches.description',
    defaultMessage: 'Try a different search term or filter.',
  },
  noMatchesTitle: {
    id: 'gateways.noMatches.title',
    defaultMessage: 'No matching gateways',
  },
  provisionButton: {
    id: 'gateways.provisionButton',
    defaultMessage: 'Provision gateway',
  },
  searchPlaceholder: {
    id: 'gateways.searchPlaceholder',
    defaultMessage: 'Search gateways',
  },
  statConnected: {
    id: 'gateways.stat.connected',
    defaultMessage: 'Connected',
  },
  statDisconnected: {
    id: 'gateways.stat.disconnected',
    defaultMessage: 'Not connected',
  },
  statEnvironments: {
    id: 'gateways.stat.environments',
    defaultMessage: 'Environments',
  },
  statTotal: {
    id: 'gateways.stat.total',
    defaultMessage: 'Total gateways',
  },
  subtitle: {
    id: 'gateways.subtitle',
    defaultMessage:
      'Provision and manage self-hosted or WSO2-managed gateways to expose your APIs to clients.',
  },
  title: {
    id: 'gateways.title',
    defaultMessage: 'Gateways',
  },
});

/** Keep the fleet summary and environment groups accurate by fetching all gateways. */
const GATEWAY_PAGE_LIMIT = 100;

/** The mode each filter admits; `all` is handled before the lookup. */
const FILTER_MODE = {
  managed: 'managed',
  self: 'self-hosted',
} as const;

export function GatewaysPage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const intl = useIntl();
  const gatewaysQuery = useGateways({ limit: GATEWAY_PAGE_LIMIT });

  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<GatewayFilter>('all');
  const [view, setView] = useState<ViewMode>('grid');

  const openGateway = (gateway: Gateway) => navigate(routes.gateway(orgHandle, gateway.id ?? ''));
  const provision = () => navigate(routes.newGateway(orgHandle));

  const gateways = useMemo(() => gatewaysQuery.data?.list ?? [], [gatewaysQuery.data]);

  // Client-side filtering: environment groups and summary tiles are computed
  // over the whole collection, not a filtered subset.
  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    return gateways.filter((gateway) => {
      const matchesTerm =
        !term || gatewaySearchFields(gateway).some((field) => field.toLowerCase().includes(term));
      const matchesFilter = filter === 'all' || gatewayMode(gateway) === FILTER_MODE[filter];
      return matchesTerm && matchesFilter;
    });
  }, [gateways, search, filter]);

  // Search and mode filtering are client-side, so an empty collection means
  // the org has no gateways.
  const isFirstRun = gateways.length === 0;

  const connectedCount = gateways.filter((gateway) => gateway.isActive).length;
  const environmentCount = groupGatewaysByEnvironment(gateways).length;
  const groups = groupGatewaysByEnvironment(filtered);

  // Use `isPending` to avoid flashing empty state during org resolution.
  if (gatewaysQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (gatewaysQuery.error) {
    return <ErrorState message={intl.formatMessage(messages.errorMessage)} />;
  }

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
        {/* Hidden on first run to avoid duplicate provision actions. */}
        {!isFirstRun && (
          <PageTitle.Actions>
            <Button onClick={provision} startIcon={<Plus />} variant="contained">
              <FormattedMessage {...messages.provisionButton} />
            </Button>
          </PageTitle.Actions>
        )}
      </PageTitle>

      {isFirstRun ? (
        <EmptyState
          actionIcon={<Plus />}
          actionLabel={intl.formatMessage(messages.emptyAction)}
          description={intl.formatMessage(messages.emptyDescription)}
          illustration={<GatewayIllustration />}
          onAction={provision}
          title={intl.formatMessage(messages.emptyTitle)}
        />
      ) : (
        <Stack spacing={3}>
          {/* Summary tiles: the fleet at a glance, before any filtering. */}
          <Grid container spacing={2}>
            <Grid size={{ md: 3, xs: 6 }}>
              <StatCard
                icon={<Network size={24} />}
                label={intl.formatMessage(messages.statTotal)}
                value={gateways.length}
              />
            </Grid>
            <Grid size={{ md: 3, xs: 6 }}>
              <StatCard
                icon={<Wifi size={24} />}
                iconColor="success"
                label={intl.formatMessage(messages.statConnected)}
                value={connectedCount}
              />
            </Grid>
            <Grid size={{ md: 3, xs: 6 }}>
              <StatCard
                icon={<WifiOff size={24} />}
                iconColor="warning"
                label={intl.formatMessage(messages.statDisconnected)}
                value={gateways.length - connectedCount}
              />
            </Grid>
            <Grid size={{ md: 3, xs: 6 }}>
              <StatCard
                icon={<Layers size={24} />}
                iconColor="info"
                label={intl.formatMessage(messages.statEnvironments)}
                value={environmentCount}
              />
            </Grid>
          </Grid>

          {/* Full-bleed search: the field owns its own row across the page. */}
          <SearchBar
            fullWidth
            onChange={(event) => setSearch(event.target.value)}
            placeholder={intl.formatMessage(messages.searchPlaceholder)}
            value={search}
          />

          <Box
            sx={{
              alignItems: 'center',
              display: 'flex',
              flexWrap: 'wrap',
              gap: 2,
              justifyContent: 'space-between',
            }}
          >
            <Typography variant="h6">
              <FormattedMessage {...messages.gatewayCount} values={{ count: filtered.length }} />
            </Typography>
            <Stack alignItems="center" direction="row" spacing={1.5}>
              <ToggleButtonGroup
                exclusive
                onChange={(_event, next: GatewayFilter | null) => {
                  if (next) setFilter(next);
                }}
                size="small"
                value={filter}
              >
                <ToggleButton value="all">
                  <FormattedMessage {...messages.filterAll} />
                </ToggleButton>
                <ToggleButton value="managed">
                  <FormattedMessage {...messages.filterManaged} />
                </ToggleButton>
                <ToggleButton value="self">
                  <FormattedMessage {...messages.filterSelfHosted} />
                </ToggleButton>
              </ToggleButtonGroup>
              <ToggleButtonGroup
                exclusive
                onChange={(_event, next: ViewMode | null) => {
                  if (next) setView(next);
                }}
                size="small"
                value={view}
              >
                <ToggleButton aria-label={intl.formatMessage(messages.gridView)} value="grid">
                  <LayoutGrid size={16} />
                </ToggleButton>
                <ToggleButton aria-label={intl.formatMessage(messages.listView)} value="list">
                  <List size={16} />
                </ToggleButton>
              </ToggleButtonGroup>
            </Stack>
          </Box>

          {groups.length === 0 ? (
            <EmptyState
              description={intl.formatMessage(messages.noMatchesDescription)}
              title={intl.formatMessage(messages.noMatchesTitle)}
            />
          ) : (
            groups.map((group) => (
              <Stack key={group.environment.id} spacing={2}>
                <Stack alignItems="center" direction="row" spacing={1.5}>
                  <Shrub size={20} />
                  <Typography variant="h6">{group.environment.name}</Typography>
                  <Chip label={group.gateways.length} size="small" variant="outlined" />
                </Stack>
                {/* The toggle changes only how a group renders its gateways */}
                {view === 'grid' ? (
                  <GatewayGridView gateways={group.gateways} onOpen={openGateway} />
                ) : (
                  <GatewayListView gateways={group.gateways} onOpen={openGateway} />
                )}
              </Stack>
            ))
          )}
        </Stack>
      )}
    </>
  );
}
