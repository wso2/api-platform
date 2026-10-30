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
import { Box, Button, Card, PageTitle, SearchBar, Stack, Typography } from '@wso2/oxygen-ui';
import { PanelsTopLeft, Plus } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate, useParams } from 'react-router-dom';

import { useApiPortals } from '@/api/resources/apiPortals';
import { AppPage } from '@/components/AppPage';
import { openableProps } from '@/components/openable';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { Can } from '@/permissions';
import { routes } from '@/routes/paths';
import { focusRingSx } from '@/theme';
import { GatewayUpdatedLabel } from '../gateways/components/gatewayChips';

const messages = defineMessages({
  columnName: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.column.name',
    defaultMessage: 'Portal',
    description: 'Column header over the portal name and handle.',
  },
  columnUpdated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.column.updated',
    defaultMessage: 'Updated',
    description: 'Column header over the last-updated timestamp.',
  },
  columnUrl: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.column.url',
    defaultMessage: 'URL',
    description: 'Column header over the portal address.',
  },
  count: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.count',
    defaultMessage: '{count, plural, one {# portal} other {# portals}}',
    description: 'Heading above the listing, counting every match.',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.empty.description',
    defaultMessage:
      'Register an API Portal you run so you can publish APIs to it for developers to discover and subscribe to.',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.empty.title',
    defaultMessage: 'Register your first API Portal',
    description: 'First-run prompt shown to an organization with no API Portals.',
  },
  errorMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.error',
    defaultMessage: 'Unable to load API Portals',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.loading',
    defaultMessage: 'Loading API Portals',
  },
  noMatchesDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.noMatches.description',
    defaultMessage: 'Try a different search term.',
  },
  noMatchesTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.noMatches.title',
    defaultMessage: 'No matching portals',
  },
  register: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.register',
    defaultMessage: 'Register portal',
  },
  searchPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.searchPlaceholder',
    defaultMessage: 'Search portals',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.subtitle',
    defaultMessage: 'Register the API Portals you run and connect them to the control plane.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalsPage.title',
    defaultMessage: 'API Portals',
  },
});

/** An org runs a handful of portals; one page keeps search client-side. */
const PORTAL_PAGE_LIMIT = 100;

/** Shared by the header and every row so a label never drifts from its cells. */
const rowGridSx = {
  alignItems: 'center',
  display: 'grid',
  gap: 2,
  gridTemplateColumns: { xs: 'minmax(0, 1fr)', md: 'minmax(0, 1fr) minmax(0, 1fr) 180px' },
} as const;

const wideOnlySx = { display: { md: 'block', xs: 'none' } } as const;
const wideOnlyFlexSx = { display: { md: 'flex', xs: 'none' } } as const;
const headerLabelSx = { fontWeight: 700 } as const;

export function ApiPortalsPage() {
  return (
    <AppPage>
      <ApiPortalsPageContent />
    </AppPage>
  );
}

function ApiPortalsPageContent() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const intl = useIntl();
  const portalsQuery = useApiPortals({ limit: PORTAL_PAGE_LIMIT });
  const [search, setSearch] = useState('');

  const portals = useMemo(() => portalsQuery.data?.list ?? [], [portalsQuery.data]);

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return portals;
    return portals.filter((portal) =>
      [portal.name, portal.id, portal.url].some((field) => field.toLowerCase().includes(term)),
    );
  }, [portals, search]);

  const register = () => navigate(routes.newOrgApiPortal(orgHandle));

  // `isPending` rather than `isLoading`: the query waits on org resolution.
  if (portalsQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (portalsQuery.error) {
    return <ErrorState message={intl.formatMessage(messages.errorMessage)} />;
  }

  const isFirstRun = portals.length === 0;

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
        {/* Hidden on first run, where the empty state carries the same action. */}
        {!isFirstRun && (
          <PageTitle.Actions>
            <Can do="CreateApiPortal" denied="disable">
              <Button onClick={register} startIcon={<Plus />} variant="contained">
                <FormattedMessage {...messages.register} />
              </Button>
            </Can>
          </PageTitle.Actions>
        )}
      </PageTitle>

      {isFirstRun ? (
        <EmptyState
          actionIcon={<Plus />}
          actionLabel={intl.formatMessage(messages.register)}
          description={intl.formatMessage(messages.emptyDescription)}
          onAction={register}
          operationId="CreateApiPortal"
          title={intl.formatMessage(messages.emptyTitle)}
        />
      ) : (
        <Stack spacing={3}>
          <SearchBar
            fullWidth
            onChange={(event) => setSearch(event.target.value)}
            placeholder={intl.formatMessage(messages.searchPlaceholder)}
            value={search}
          />

          <Typography variant="h6">
            <FormattedMessage {...messages.count} values={{ count: filtered.length }} />
          </Typography>

          {filtered.length === 0 ? (
            <EmptyState
              description={intl.formatMessage(messages.noMatchesDescription)}
              title={intl.formatMessage(messages.noMatchesTitle)}
            />
          ) : (
            <Card data-testid="api-portal-list" variant="outlined">
              <Box sx={{ ...rowGridSx, bgcolor: 'action.hover', px: 2.5, py: 1.25 }}>
                <Typography color="text.secondary" sx={headerLabelSx} variant="caption">
                  <FormattedMessage {...messages.columnName} />
                </Typography>
                <Typography
                  color="text.secondary"
                  sx={{ ...wideOnlySx, ...headerLabelSx }}
                  variant="caption"
                >
                  <FormattedMessage {...messages.columnUrl} />
                </Typography>
                <Typography
                  color="text.secondary"
                  sx={{ ...wideOnlySx, ...headerLabelSx }}
                  variant="caption"
                >
                  <FormattedMessage {...messages.columnUpdated} />
                </Typography>
              </Box>
              {filtered.map((portal) => (
                <Box
                  key={portal.id}
                  {...openableProps(intl, portal.name, () =>
                    navigate(routes.orgApiPortal(orgHandle, portal.id)),
                  )}
                  sx={(theme) => ({
                    borderBottom: `${theme.border.width} ${theme.border.style}`,
                    borderColor: 'divider',
                    cursor: 'pointer',
                    px: 2.5,
                    py: 1.75,
                    transition: theme.transitions.create('background-color'),
                    ...rowGridSx,
                    ...focusRingSx(theme),
                    '&:hover': { bgcolor: 'action.hover' },
                    '&:last-of-type': { borderBottom: 0 },
                  })}
                >
                  <Stack alignItems="center" direction="row" spacing={1.5} sx={{ minWidth: 0 }}>
                    <Box sx={{ color: 'text.secondary', display: 'flex', flexShrink: 0 }}>
                      <PanelsTopLeft size={20} />
                    </Box>
                    <Box sx={{ minWidth: 0 }}>
                      <Typography component="div" noWrap sx={{ fontWeight: 600 }} variant="subtitle2">
                        {portal.name}
                      </Typography>
                      <Typography color="text.secondary" component="div" noWrap variant="caption">
                        {portal.id}
                      </Typography>
                    </Box>
                  </Stack>
                  <Typography
                    color="text.secondary"
                    noWrap
                    sx={{ ...wideOnlySx, minWidth: 0 }}
                    variant="body2"
                  >
                    {portal.url}
                  </Typography>
                  <Box sx={wideOnlyFlexSx}>
                    <GatewayUpdatedLabel timestamp={portal.updatedAt || portal.createdAt} />
                  </Box>
                </Box>
              ))}
            </Card>
          )}
        </Stack>
      )}
    </>
  );
}
