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
import { Card, Chip, ListingTable, Stack, Typography } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useGatewayManifest } from '@/api/resources/gateways';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';

const messages = defineMessages({
  columnDescription: {
    id: 'gateways.detail.Policies.column.description',
    defaultMessage: 'Description',
  },
  columnName: {
    id: 'gateways.detail.Policies.column.name',
    defaultMessage: 'Name',
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
  subtitle: {
    id: 'gateways.detail.Policies.subtitle',
    defaultMessage:
      'Mediation policies installed on this gateway, as reported by its last manifest sync.',
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

/** Widths that stop the version and type columns from stealing the description's room. */
const VERSION_COLUMN_WIDTH = 120;
const TYPE_COLUMN_WIDTH = 160;

type SortDirection = 'asc' | 'desc';

export type GatewayPoliciesPanelProps = {
  gatewayId: string;
};

/**
 * The policies a gateway is currently running, read from its manifest.
 *
 * A read-only listing on purpose: what is installed on a gateway is decided by
 * the manifest the control plane hands it, not by an action taken here, so the
 * table has no row actions and nothing to edit.
 *
 * Sorting is client-side because the manifest arrives whole — there is no
 * paged endpoint behind it, so a round trip per sort click would re-fetch the
 * same document to reorder rows the browser already holds.
 */
export function GatewayPoliciesPanel({ gatewayId }: GatewayPoliciesPanelProps) {
  const intl = useIntl();
  const manifestQuery = useGatewayManifest(gatewayId);

  const [sortDirection, setSortDirection] = useState<SortDirection>('asc');

  const policies = useMemo(() => {
    const list = manifestQuery.data?.policies ?? [];
    // `localeCompare` keeps accented and non-Latin names sorted naturally.
    return [...list].sort((left, right) =>
      sortDirection === 'asc'
        ? left.name.localeCompare(right.name)
        : right.name.localeCompare(left.name),
    );
  }, [manifestQuery.data, sortDirection]);

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
    <ListingTable.Provider
      onSortChange={(_field, direction) => setSortDirection(direction)}
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
            </ListingTable.Row>
          </ListingTable.Head>
          <ListingTable.Body>
            {policies.map((policy) => (
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
                      {policy.description}
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
              </ListingTable.Row>
            ))}
          </ListingTable.Body>
        </ListingTable>
      </ListingTable.Container>
    </ListingTable.Provider>
  );

  return (
    <Card sx={{ p: 3 }}>
      <Stack spacing={3}>
        <Stack spacing={0.5}>
          <Typography sx={{ fontWeight: 700 }} variant="h5">
            <FormattedMessage {...messages.title} />
          </Typography>
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.subtitle} />
          </Typography>
        </Stack>
        {body}
      </Stack>
    </Card>
  );
}
