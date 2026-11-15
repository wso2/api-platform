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
  Card,
  Chip,
  ListingTable,
  SearchBar,
  Skeleton,
  Stack,
  TablePagination,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { SearchX } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import {
  useAllGatewayCustomPolicies,
  useSyncCustomPolicy,
} from '@/api/resources/gatewayCustomPolicies';
import { useGatewayManifest } from '@/api/resources/gateways';
import { EmptyResults } from '@/components/common/EmptyResults';
import { TruncatedText } from '@/components/common/TruncatedText';
import { useNotifications } from '@/components/Notifications';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { useTrackFeature } from '@/featureUsage';
import { Can, useCan } from '@/permissions';
import { customPolicySyncStatus, type CustomPolicySyncStatus } from '../utils/customPolicySync';

const messages = defineMessages({
  columnDescription: {
    id: 'gateways.detail.Policies.column.description',
    defaultMessage: 'Description',
  },
  columnName: {
    id: 'gateways.detail.Policies.column.name',
    defaultMessage: 'Name',
  },
  columnOrganization: {
    id: 'gateways.detail.Policies.column.organization',
    defaultMessage: 'Sync status',
    description:
      'Column stating whether a custom policy has been synced to the organization, which is what lets its APIs use it.',
  },
  columnType: {
    id: 'gateways.detail.Policies.column.type',
    defaultMessage: 'Policy Type',
    description: 'Column stating where a policy came from — the Policy Hub, or a custom upload.',
  },
  columnVersion: {
    id: 'gateways.detail.Policies.column.version',
    defaultMessage: 'Version',
  },
  emptyDescription: {
    id: 'gateways.detail.Policies.empty.description',
    defaultMessage:
      'This gateway has not reported a manifest yet. Once it connects and syncs, the policies it is running appear here.',
  },
  emptyTitle: {
    id: 'gateways.detail.Policies.empty.title',
    defaultMessage: 'No gateway manifest found',
  },
  errorMessage: {
    id: 'gateways.detail.Policies.error',
    defaultMessage: 'Unable to load the policies on this gateway.',
  },
  loading: {
    id: 'gateways.detail.Policies.loading',
    defaultMessage: 'Loading policies',
  },
  noDescription: {
    id: 'gateways.detail.Policies.noDescription',
    defaultMessage: 'No description',
    description:
      'Stands in for a policy description when the manifest carries none. Rendered as an absence, not a value.',
  },
  organizationVersion: {
    id: 'gateways.detail.Policies.organizationVersion',
    defaultMessage: 'Organization version: {version}',
    description:
      'Under a synced custom policy, when the organization holds a different version of it than this gateway runs. {version} is backend data, e.g. "v1.2.0".',
  },
  replacesVersion: {
    id: 'gateways.detail.Policies.replacesVersion',
    defaultMessage: 'Replaces {version}',
    description:
      'Under Sync, when syncing replaces the lower version the organization holds. {version} is backend data, e.g. "v1.2.0".',
  },
  clearFilters: {
    id: 'gateways.detail.Policies.clearFilters',
    defaultMessage: 'Clear search and filter',
  },
  filterAll: {
    id: 'gateways.detail.Policies.filter.all',
    defaultMessage: 'All',
    description: 'Policy Type filter option that shows every policy.',
  },
  filterCustom: {
    id: 'gateways.detail.Policies.filter.custom',
    defaultMessage: 'Custom',
    description: 'Policy Type filter option that shows only custom policies.',
  },
  filterLabel: {
    id: 'gateways.detail.Policies.filter.label',
    defaultMessage: 'Filter by policy type',
  },
  filterPolicyHub: {
    id: 'gateways.detail.Policies.filter.policyHub',
    defaultMessage: 'Policy Hub',
    description:
      'Policy Type filter option that shows only Policy Hub policies. Product name — do not translate “Policy Hub”.',
  },
  nextPage: {
    id: 'gateways.detail.Policies.pagination.next',
    defaultMessage: 'Next page',
  },
  noMatchesDescription: {
    id: 'gateways.detail.Policies.noMatches.description',
    defaultMessage: 'Try a different search term or filter.',
  },
  noMatchesTitle: {
    id: 'gateways.detail.Policies.noMatches.title',
    defaultMessage: 'No matching policies',
  },
  pageRange: {
    id: 'gateways.detail.Policies.pagination.range',
    defaultMessage: '{from}–{to} of {count, number}',
    description: 'Pager summary: the policy rows shown on this page out of the total.',
  },
  previousPage: {
    id: 'gateways.detail.Policies.pagination.previous',
    defaultMessage: 'Previous page',
  },
  rowsPerPage: {
    id: 'gateways.detail.Policies.pagination.rowsPerPage',
    defaultMessage: 'Policies per page',
  },
  searchPlaceholder: {
    id: 'gateways.detail.Policies.searchPlaceholder',
    defaultMessage: 'Search policies',
  },
  subtitle: {
    id: 'gateways.detail.Policies.subtitle',
    defaultMessage: 'Policies installed on this gateway, as reported when it last connected.',
  },
  sync: {
    id: 'gateways.detail.Policies.sync',
    defaultMessage: 'Sync',
    description:
      'Button copying a custom policy from this gateway into the organization, so that its APIs can use the policy. Verb.',
  },
  syncHint: {
    id: 'gateways.detail.Policies.syncHint',
    defaultMessage: 'Sync a custom policy to make it available to your organization’s APIs.',
  },
  syncNotNeeded: {
    id: 'gateways.detail.Policies.syncNotNeeded',
    defaultMessage: 'Built-in',
    description: 'Sync status value for a Policy Hub policy: APIs can use it without a sync.',
  },
  syncStatusUnavailable: {
    id: 'gateways.detail.Policies.syncStatusUnavailable',
    defaultMessage:
      'The organization’s custom policies could not be loaded, so whether each one is synced is not shown.',
  },
  syncSucceeded: {
    id: 'gateways.detail.Policies.syncSucceeded',
    defaultMessage: 'Synced {policyName} {version} to the organization.',
    description:
      'Toast after a sync. {policyName} and {version} are backend data, e.g. "hmac-signer" and "v1.0.0".',
  },
  synced: {
    id: 'gateways.detail.Policies.synced',
    defaultMessage: 'Synced',
    description: 'Organization column value: the custom policy is in the organization. Adjective.',
  },
  syncing: {
    id: 'gateways.detail.Policies.syncing',
    defaultMessage: 'Syncing',
    description: 'Label of the Sync button while its request is in flight.',
  },
  syncUnknown: {
    id: 'gateways.detail.Policies.syncUnknown',
    defaultMessage: 'Unknown',
    description:
      'Organization column value when the organization’s custom policies could not be read.',
  },
  title: {
    id: 'gateways.detail.Policies.title',
    defaultMessage: 'Policies',
  },
  typeCustom: {
    id: 'gateways.detail.Policies.type.custom',
    defaultMessage: 'Custom',
    description: 'Policy Type value for a policy the organization installed itself.',
  },
  typePolicyHub: {
    id: 'gateways.detail.Policies.type.policyHub',
    defaultMessage: 'Policy Hub',
    description:
      'Policy Type value for a policy shipped by the platform’s own catalog. Product name — do not translate “Policy Hub”.',
  },
});

/** The only column the table sorts on, and the order it opens in. */
const SORT_FIELD = 'name';

/** Widths that stop the version, type and sync columns from stealing the description's room. */
const VERSION_COLUMN_WIDTH = 120;
const TYPE_COLUMN_WIDTH = 160;
const ORGANIZATION_COLUMN_WIDTH = 200;

const PAGE_SIZE_OPTIONS = [5, 10, 25, 50];

/** Lines of a description a row shows; the full text is in a tooltip. */
const DESCRIPTION_MAX_LINES = 2;

type SortDirection = 'asc' | 'desc';

type TypeFilter = 'all' | 'custom' | 'policyHub';

type ManifestPolicy = {
  description?: string;
  isCustomPolicy: boolean;
  name: string;
  version: string;
};

export type GatewayPoliciesPanelProps = {
  gatewayId: string;
  /** `embedded` drops the card and heading, for a host that frames the panel itself. */
  variant?: 'card' | 'embedded';
};

/**
 * The policies a gateway is currently running, read from its manifest, with
 * Sync for the custom policies the organization does not yet hold.
 *
 * Sorting, search, filtering and paging are client-side: the manifest arrives
 * whole, with no paged endpoint behind it.
 */
export function GatewayPoliciesPanel({ gatewayId, variant = 'card' }: GatewayPoliciesPanelProps) {
  const intl = useIntl();
  const manifestQuery = useGatewayManifest(gatewayId);

  const [sortDirection, setSortDirection] = useState<SortDirection>('asc');
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState<TypeFilter>('all');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(PAGE_SIZE_OPTIONS[0]);

  const policies = useMemo<ManifestPolicy[]>(() => {
    const list = manifestQuery.data?.policies ?? [];
    // `localeCompare` keeps accented and non-Latin names sorted naturally.
    return [...list].sort((left, right) =>
      sortDirection === 'asc'
        ? left.name.localeCompare(right.name)
        : right.name.localeCompare(left.name),
    );
  }, [manifestQuery.data, sortDirection]);

  const hasCustomPolicies = policies.some((policy) => policy.isCustomPolicy);
  const canReadOrganizationPolicies = useCan('ListGatewayCustomPolicies');
  const organizationPoliciesQuery = useAllGatewayCustomPolicies({
    enabled: hasCustomPolicies && canReadOrganizationPolicies,
  });
  const organizationPolicies = organizationPoliciesQuery.data?.list;
  const syncStatusUnknown =
    !canReadOrganizationPolicies || Boolean(organizationPoliciesQuery.error);

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    return policies.filter(
      (policy) =>
        (typeFilter === 'all' || policy.isCustomPolicy === (typeFilter === 'custom')) &&
        (!query ||
          policy.name.toLowerCase().includes(query) ||
          Boolean(policy.description?.toLowerCase().includes(query))),
    );
  }, [policies, search, typeFilter]);

  const lastPage = Math.max(0, Math.ceil(filtered.length / rowsPerPage) - 1);
  const currentPage = Math.min(page, lastPage);
  const pageRows = filtered.slice(currentPage * rowsPerPage, (currentPage + 1) * rowsPerPage);

  const clearFilters = () => {
    setSearch('');
    setTypeFilter('all');
    setPage(0);
  };

  const toolbar = (
    <Box sx={{ alignItems: 'center', display: 'flex', flexWrap: 'wrap', gap: 2 }}>
      <Box sx={{ flex: '1 1 280px' }}>
        <SearchBar
          fullWidth
          onChange={(event) => {
            setSearch(event.target.value);
            setPage(0);
          }}
          placeholder={intl.formatMessage(messages.searchPlaceholder)}
          value={search}
        />
      </Box>
      {hasCustomPolicies && (
        <ToggleButtonGroup
          aria-label={intl.formatMessage(messages.filterLabel)}
          exclusive
          onChange={(_event, next: TypeFilter | null) => {
            if (!next) return;
            setTypeFilter(next);
            setPage(0);
          }}
          size="small"
          value={typeFilter}
        >
          <ToggleButton sx={{ textTransform: 'none' }} value="all">
            <FormattedMessage {...messages.filterAll} />
          </ToggleButton>
          <ToggleButton sx={{ textTransform: 'none' }} value="custom">
            <FormattedMessage {...messages.filterCustom} />
          </ToggleButton>
          <ToggleButton sx={{ textTransform: 'none' }} value="policyHub">
            <FormattedMessage {...messages.filterPolicyHub} />
          </ToggleButton>
        </ToggleButtonGroup>
      )}
    </Box>
  );

  const table = (
    <ListingTable.Provider
      onSortChange={(_field, direction) => {
        setSortDirection(direction);
        setPage(0);
      }}
      sortDirection={sortDirection}
      sortField={SORT_FIELD}
    >
      <ListingTable.Container>
        <ListingTable>
          <ListingTable.Head>
            <ListingTable.Row>
              <ListingTable.Cell>
                <ListingTable.SortLabel field={SORT_FIELD}>
                  <FormattedMessage {...messages.columnName} />
                </ListingTable.SortLabel>
              </ListingTable.Cell>
              <ListingTable.Cell sx={{ width: VERSION_COLUMN_WIDTH }}>
                <FormattedMessage {...messages.columnVersion} />
              </ListingTable.Cell>
              <ListingTable.Cell>
                <FormattedMessage {...messages.columnDescription} />
              </ListingTable.Cell>
              <ListingTable.Cell sx={{ width: TYPE_COLUMN_WIDTH }}>
                <FormattedMessage {...messages.columnType} />
              </ListingTable.Cell>
              {hasCustomPolicies && (
                <ListingTable.Cell sx={{ width: ORGANIZATION_COLUMN_WIDTH }}>
                  <FormattedMessage {...messages.columnOrganization} />
                </ListingTable.Cell>
              )}
            </ListingTable.Row>
          </ListingTable.Head>
          <ListingTable.Body>
            {pageRows.map((policy) => (
              <ListingTable.Row key={`${policy.name}@${policy.version}`}>
                <ListingTable.Cell>
                  {/* Policy names are data from the manifest, never copy. */}
                  <Typography sx={{ fontWeight: 600 }} variant="body2">
                    {policy.name}
                  </Typography>
                </ListingTable.Cell>
                <ListingTable.Cell>
                  <Chip
                    label={policy.version}
                    size="small"
                    sx={{ typography: 'caption' }}
                    variant="outlined"
                  />
                </ListingTable.Cell>
                <ListingTable.Cell>
                  {policy.description ? (
                    <Typography color="text.secondary" variant="body2">
                      <TruncatedText maxLines={DESCRIPTION_MAX_LINES} text={policy.description} />
                    </Typography>
                  ) : (
                    <Typography color="text.disabled" sx={{ fontStyle: 'italic' }} variant="body2">
                      <FormattedMessage {...messages.noDescription} />
                    </Typography>
                  )}
                </ListingTable.Cell>
                <ListingTable.Cell>
                  <Chip
                    label={intl.formatMessage(
                      policy.isCustomPolicy ? messages.typeCustom : messages.typePolicyHub,
                    )}
                    size="small"
                    sx={{ typography: 'caption' }}
                    variant="filled"
                  />
                </ListingTable.Cell>
                {hasCustomPolicies && (
                  <ListingTable.Cell>
                    {!policy.isCustomPolicy ? (
                      <Typography color="text.secondary" variant="body2">
                        <FormattedMessage {...messages.syncNotNeeded} />
                      </Typography>
                    ) : syncStatusUnknown ? (
                      <Typography color="text.secondary" variant="body2">
                        <FormattedMessage {...messages.syncUnknown} />
                      </Typography>
                    ) : !organizationPolicies ? (
                      <Skeleton variant="text" width={72} />
                    ) : (
                      <CustomPolicySyncCell
                        gatewayId={gatewayId}
                        onSyncFailed={() => void organizationPoliciesQuery.refetch()}
                        policy={policy}
                        status={customPolicySyncStatus(policy, organizationPolicies)}
                      />
                    )}
                  </ListingTable.Cell>
                )}
              </ListingTable.Row>
            ))}
          </ListingTable.Body>
        </ListingTable>
      </ListingTable.Container>
    </ListingTable.Provider>
  );

  // Use `isPending` so we don't flash the empty state while org data loads.
  const body = manifestQuery.isPending ? (
    <LoadingState label={intl.formatMessage(messages.loading)} />
  ) : manifestQuery.error ? (
    <ErrorState message={intl.formatMessage(messages.errorMessage)} />
  ) : policies.length === 0 ? (
    <EmptyState
      description={intl.formatMessage(messages.emptyDescription)}
      title={intl.formatMessage(messages.emptyTitle)}
    />
  ) : (
    <Stack spacing={2.5}>
      {toolbar}
      {filtered.length === 0 ? (
        <EmptyResults
          actionLabel={intl.formatMessage(messages.clearFilters)}
          description={intl.formatMessage(messages.noMatchesDescription)}
          illustration={<SearchX size={34} strokeWidth={1.5} />}
          onAction={clearFilters}
          title={intl.formatMessage(messages.noMatchesTitle)}
        />
      ) : (
        table
      )}
      {filtered.length > PAGE_SIZE_OPTIONS[0] && (
        <TablePagination
          component="div"
          count={filtered.length}
          getItemAriaLabel={(type) =>
            intl.formatMessage(type === 'next' ? messages.nextPage : messages.previousPage)
          }
          labelDisplayedRows={({ count, from, to }) =>
            intl.formatMessage(messages.pageRange, { count, from, to })
          }
          labelRowsPerPage={intl.formatMessage(messages.rowsPerPage)}
          onPageChange={(_, nextPage) => setPage(nextPage)}
          onRowsPerPageChange={(event) => {
            setRowsPerPage(parseInt(event.target.value, 10));
            setPage(0);
          }}
          page={currentPage}
          rowsPerPage={rowsPerPage}
          rowsPerPageOptions={PAGE_SIZE_OPTIONS}
        />
      )}
    </Stack>
  );

  const showSyncHint = hasCustomPolicies && !manifestQuery.isPending && !manifestQuery.error;
  const intro = (
    <Stack spacing={0.5}>
      {variant === 'card' && (
        <Typography sx={{ fontWeight: 700 }} variant="h5">
          <FormattedMessage {...messages.title} />
        </Typography>
      )}
      <Typography color="text.secondary" variant="body2">
        <FormattedMessage {...messages.subtitle} />
      </Typography>
      {showSyncHint && (
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage {...messages.syncHint} />
        </Typography>
      )}
      {showSyncHint && canReadOrganizationPolicies && organizationPoliciesQuery.error && (
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage {...messages.syncStatusUnavailable} />
        </Typography>
      )}
    </Stack>
  );

  const content = (
    <Stack spacing={3}>
      {intro}
      {body}
    </Stack>
  );

  return variant === 'card' ? <Card sx={{ p: 3 }}>{content}</Card> : content;
}

/** The Organization cell of a custom policy: its sync status, and Sync when the server would accept one. */
function CustomPolicySyncCell({
  gatewayId,
  onSyncFailed,
  policy,
  status,
}: {
  gatewayId: string;
  onSyncFailed: () => void;
  policy: ManifestPolicy;
  status: CustomPolicySyncStatus;
}) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const trackFeature = useTrackFeature();
  const syncMutation = useSyncCustomPolicy();

  const sync = () => {
    trackFeature('apip.gateway.custom-policy-sync-click', {
      sync_kind: status.state === 'update-available' ? 'update' : 'add',
    });
    syncMutation.mutate(
      { gatewayId, policyName: policy.name, policyVersion: policy.version },
      {
        onError: onSyncFailed,
        onSuccess: (synced) =>
          notify(
            intl.formatMessage(messages.syncSucceeded, {
              policyName: synced.name,
              version: synced.version,
            }),
            'success',
          ),
      },
    );
  };

  const heldVersion =
    status.state !== 'not-synced' && status.organizationVersion !== policy.version
      ? status.organizationVersion
      : undefined;

  return (
    <Stack alignItems="flex-start" spacing={0.5}>
      {status.state === 'synced' ? (
        <Chip
          color="success"
          label={intl.formatMessage(messages.synced)}
          size="small"
          sx={{ typography: 'caption' }}
          variant="outlined"
        />
      ) : (
        <Can denied="disable" do="SyncCustomPolicy">
          <Button disabled={syncMutation.isPending} onClick={sync} size="small" variant="outlined">
            <FormattedMessage {...(syncMutation.isPending ? messages.syncing : messages.sync)} />
          </Button>
        </Can>
      )}
      {heldVersion && (
        <Typography color="text.secondary" variant="caption">
          <FormattedMessage
            {...(status.state === 'update-available'
              ? messages.replacesVersion
              : messages.organizationVersion)}
            values={{ version: heldVersion }}
          />
        </Typography>
      )}
    </Stack>
  );
}
