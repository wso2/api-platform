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
  Checkbox,
  ListingTable,
  SearchBar,
  Stack,
  TablePagination,
  Typography,
} from '@wso2/oxygen-ui';
import { useEffect, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl, type IntlShape } from 'react-intl';

import { useMetersById } from '@/api/resources/meters';
import {
  useSubscriptionPlans,
  useSubscriptionPlansById,
  type SubscriptionPlan,
} from '@/api/resources/subscriptionPlans';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { pricingSummary, summaryMeterIds } from '../../settings/components/pricingFormat';

const PAGE_SIZE_OPTIONS = [5, 10, 15, 20];
const SEARCH_DEBOUNCE_MS = 300;

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.loading',
    defaultMessage: 'Loading subscription plans',
  },
  error: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.error',
    defaultMessage: 'Unable to load subscription plans.',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.emptyTitle',
    defaultMessage: 'No active subscription plans',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.emptyDescription',
    defaultMessage: 'Create or activate plans in Settings.',
  },
  noMatchesTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.noMatches.title',
    defaultMessage: 'No matching plans',
  },
  noMatchesDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.noMatches.description',
    defaultMessage: 'Try a different search term.',
  },
  noneOffered: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.noneOffered',
    defaultMessage: 'No subscription plans',
    description: 'Shown on the published version when it offers no subscription plans.',
  },
  columnName: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.columnName',
    defaultMessage: 'Plan',
  },
  columnRateLimit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.columnRateLimit',
    defaultMessage: 'Rate limit',
  },
  columnPricing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.columnPricing',
    defaultMessage: 'Pricing',
  },
  rateLimit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.rateLimit',
    defaultMessage:
      '{count, number} requests per {amount, plural, one {{unit, select, MINUTE {minute} HOUR {hour} DAY {day} MONTH {month} other {}}} other {# {unit, select, MINUTE {minutes} HOUR {hours} DAY {days} MONTH {months} other {}}}}',
    description:
      'A plan\'s rate limit, e.g. "1,000 requests per hour" or "500 requests per 2 days".',
  },
  unlimited: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.unlimited',
    defaultMessage: 'Unlimited',
    description: 'Rate limit of a plan that sets none.',
  },
  unavailable: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.unavailable',
    defaultMessage: 'No longer available',
    description:
      'Under a selected plan that has since been deactivated or deleted, so it cannot be offered.',
  },
  searchPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.searchPlaceholder',
    defaultMessage: 'Search plans',
  },
  nextPage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.pagination.next',
    defaultMessage: 'Next page',
  },
  previousPage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.pagination.previous',
    defaultMessage: 'Previous page',
  },
  pageRange: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.pagination.range',
    defaultMessage: '{from}–{to} of {count, number}',
    description: 'Pager summary: the plan rows shown on this page out of the active plans.',
  },
  rowsPerPage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.SubscriptionPlansTab.pagination.rowsPerPage',
    defaultMessage: 'Plans per page',
  },
});

export type SubscriptionPlansTabProps = {
  disabled?: boolean;
  /** Lists only the plans in `selected`, with nothing to pick; `onChange` is then never called. */
  readOnly?: boolean;
  onChange?: (subscriptionPlanIds: string[]) => void;
  /** Handles of the plans offered with the API. */
  selected: readonly string[];
};

/**
 * One row: a plan's handle, the plan when the organization still has it, and whether it can be
 * offered.
 */
type PlanRow = { available: boolean; id: string; plan?: SubscriptionPlan };

/** A plan's rate limit in words; a plan has at most one. */
const rateLimitText = (intl: IntlShape, plan: SubscriptionPlan | undefined): string => {
  const limit = plan?.limits?.[0];
  if (!limit) return plan ? intl.formatMessage(messages.unlimited) : '';
  return intl.formatMessage(messages.rateLimit, {
    amount: limit.timeAmount ?? 1,
    count: limit.limitCount,
    unit: limit.timeUnit,
  });
};

/**
 * "Subscription Plans": which of the organization's active plans the API Portal offers with this
 * API.
 *   - The plans are shown a page at a time, with a search. Offering none is allowed.
 *   - A picked plan stays picked while other pages are shown.
 *   - A plan picked earlier that can no longer be offered (deactivated or deleted since) is listed
 *     first on every page, so it can be seen and cleared.
 *   - Read-only, the tab lists just the offered plans, in the order saved.
 */
export function SubscriptionPlansTab(props: SubscriptionPlansTabProps) {
  return props.readOnly ? <OfferedPlans selected={props.selected} /> : <PlanPicker {...props} />;
}

function PlanPicker({ disabled, onChange, selected }: SubscriptionPlansTabProps) {
  const intl = useIntl();
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(PAGE_SIZE_OPTIONS[1]);
  const debouncedSearch = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS);

  const plansQuery = useSubscriptionPlans({
    limit: rowsPerPage,
    offset: page * rowsPerPage,
    query: debouncedSearch || undefined,
    status: 'ACTIVE',
  });
  // A plan the tab opened with keeps its row once cleared, even when it can no longer be offered,
  // so the row does not vanish under the pointer.
  const [opened] = useState(selected);
  const kept = [...opened, ...selected.filter((id) => !opened.includes(id))];
  const keptPlans = useSubscriptionPlansById(kept);

  const total = plansQuery.data?.pagination.total ?? 0;
  const lastPage = Math.max(0, Math.ceil(total / rowsPerPage) - 1);
  const currentPage = Math.min(page, lastPage);

  // A new search starts from the first page.
  useEffect(() => setPage(0), [debouncedSearch]);

  // Picked plans that are no longer active or were deleted, listed first so they can be cleared.
  const leftOver: PlanRow[] = kept.flatMap((id) => {
    const plan = keptPlans.plans.get(id);
    const gone = keptPlans.removed.has(id) || (plan !== undefined && plan.status !== 'ACTIVE');
    return gone ? [{ available: false, id, plan }] : [];
  });
  const rows: PlanRow[] = [
    ...leftOver,
    ...(plansQuery.data?.list ?? []).flatMap((plan) =>
      plan.id ? [{ available: true, id: plan.id, plan }] : [],
    ),
  ];
  const { meters } = useMetersById(rows.flatMap((row) => summaryMeterIds(row.plan?.pricing)));

  if (plansQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (plansQuery.error) return <ErrorState message={intl.formatMessage(messages.error)} />;

  if (total === 0 && debouncedSearch === '' && leftOver.length === 0) {
    return (
      <EmptyState
        description={intl.formatMessage(messages.emptyDescription)}
        title={intl.formatMessage(messages.emptyTitle)}
      />
    );
  }

  const chosen = new Set(selected);
  const toggle = (id: string) =>
    onChange?.(chosen.has(id) ? selected.filter((current) => current !== id) : [...selected, id]);

  return (
    <Box component="section" sx={{ p: 3 }}>
      <Stack spacing={2.5}>
        <SearchBar
          fullWidth
          onChange={(event) => setSearch(event.target.value)}
          placeholder={intl.formatMessage(messages.searchPlaceholder)}
          value={search}
        />
        {rows.length === 0 ? (
          <EmptyState
            description={intl.formatMessage(messages.noMatchesDescription)}
            title={intl.formatMessage(messages.noMatchesTitle)}
          />
        ) : (
          <PlanTable
            checkboxes={{ chosen, disabled, toggle }}
            intl={intl}
            meters={meters}
            rows={rows}
          />
        )}
        {total > PAGE_SIZE_OPTIONS[0] && (
          <TablePagination
            component="div"
            count={total}
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
    </Box>
  );
}

/** The published version's plans: the offered ones, in the order saved, each read by its id. */
function OfferedPlans({ selected }: { selected: readonly string[] }) {
  const intl = useIntl();
  const offered = useSubscriptionPlansById(selected);
  const rows: PlanRow[] = selected.map((id) => {
    const plan = offered.plans.get(id);
    return {
      available: plan?.status === 'ACTIVE' || (plan === undefined && !offered.removed.has(id)),
      id,
      plan,
    };
  });
  const { meters } = useMetersById(rows.flatMap((row) => summaryMeterIds(row.plan?.pricing)));

  if (selected.length === 0) return <EmptyState title={intl.formatMessage(messages.noneOffered)} />;
  if (offered.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  return (
    <Box component="section" sx={{ p: 3 }}>
      <PlanTable intl={intl} meters={meters} rows={rows} />
    </Box>
  );
}

type PlanTableProps = {
  /** With checkboxes, the plans picked and how to toggle one; without, the table is read-only. */
  checkboxes?: { chosen: Set<string>; disabled?: boolean; toggle: (id: string) => void };
  intl: IntlShape;
  meters: Parameters<typeof pricingSummary>[2];
  rows: PlanRow[];
};

function PlanTable({ checkboxes, intl, meters, rows }: PlanTableProps) {
  return (
    <ListingTable.Provider>
      <ListingTable.Container>
        <ListingTable>
          <ListingTable.Head>
            <ListingTable.Row>
              {checkboxes && <ListingTable.Cell padding="checkbox" />}
              <ListingTable.Cell>
                <FormattedMessage {...messages.columnName} />
              </ListingTable.Cell>
              <ListingTable.Cell>
                <FormattedMessage {...messages.columnRateLimit} />
              </ListingTable.Cell>
              <ListingTable.Cell>
                <FormattedMessage {...messages.columnPricing} />
              </ListingTable.Cell>
            </ListingTable.Row>
          </ListingTable.Head>
          <ListingTable.Body>
            {rows.map(({ available, id, plan }) => {
              const name = plan?.displayName ?? id;
              const checked = checkboxes?.chosen.has(id) ?? false;
              return (
                <ListingTable.Row key={id}>
                  {checkboxes && (
                    <ListingTable.Cell padding="checkbox">
                      <Checkbox
                        checked={checked}
                        // A plan that can no longer be offered can be cleared, not picked again.
                        disabled={checkboxes.disabled || (!available && !checked)}
                        onChange={() => checkboxes.toggle(id)}
                        slotProps={{ input: { 'aria-label': name } }}
                      />
                    </ListingTable.Cell>
                  )}
                  <ListingTable.Cell>
                    <Typography sx={{ fontWeight: 600 }} variant="body2">
                      {name}
                    </Typography>
                    {!available && (
                      <Typography color="warning.main" variant="caption">
                        <FormattedMessage {...messages.unavailable} />
                      </Typography>
                    )}
                  </ListingTable.Cell>
                  <ListingTable.Cell>
                    <Typography variant="body2">{rateLimitText(intl, plan)}</Typography>
                  </ListingTable.Cell>
                  <ListingTable.Cell>
                    <Typography
                      color={plan?.pricing ? 'text.primary' : 'text.secondary'}
                      variant="body2"
                    >
                      {plan && pricingSummary(intl, plan.pricing, meters)}
                    </Typography>
                  </ListingTable.Cell>
                </ListingTable.Row>
              );
            })}
          </ListingTable.Body>
        </ListingTable>
      </ListingTable.Container>
    </ListingTable.Provider>
  );
}
