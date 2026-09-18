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

import { Box, PageTitle } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';

import { useApiPublications, type PublicationSummaryItem } from '@/api/resources/apiPublications';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { PortalPublicationCard } from './components/PortalPublicationCard';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.ApiPortalPublicationsList.title',
    defaultMessage: 'Publish {apiName}',
    description:
      'Page heading. {apiName} is the API display name, user-supplied; do not translate it.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.ApiPortalPublicationsList.subtitle',
    defaultMessage:
      'Choose an API portal to publish this API to, or check where it is already published.',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.ApiPortalPublicationsList.loading',
    defaultMessage: 'Loading API portals',
  },
  errorMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.ApiPortalPublicationsList.errorMessage',
    defaultMessage: 'Unable to load API portals',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.ApiPortalPublicationsList.emptyTitle',
    defaultMessage: 'No API portals available',
    description: 'Shown when the organization has not registered any API Portal yet.',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.ApiPortalPublicationsList.emptyDescription',
    defaultMessage:
      'An organization admin needs to register an API portal before this API can be published.',
  },
  emptyAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.ApiPortalPublicationsList.emptyAction',
    defaultMessage: 'Manage portals',
  },
});

/**
 * Every API Portal registered in the organization, each annotated with this
 * API's own publication status — the rollup from `GET /api-publications`.
 *
 * `apiType` is hardcoded to `rest-api`: it is the only API family the publish
 * flow (and this console's `restApis` resource) supports end to end today. See
 * `Implementation_Plan.md` Slice 5/6 — `websub-api`/`webbroker-api` are declared
 * in the spec's enum but have no publish/unpublish path wired up yet.
 */
const API_TYPE = 'rest-api';

/** Portals are an org-wide, rarely-changing collection — one page is enough. */
const LIST_LIMIT = 100;

export function ApiPortalPublicationsList() {
  const intl = useIntl();
  const navigate = useNavigate();
  const { component, params } = useConsoleScope();
  const orgHandle = params.orgHandle ?? '';
  const projectHandler = params.projectHandler ?? '';
  const apiHandler = params.apiHandler ?? '';

  const publicationsQuery = useApiPublications(API_TYPE, apiHandler, { limit: LIST_LIMIT });

  const openPublication = (publication: PublicationSummaryItem) => {
    if (!publication.apiPortalId) return;
    navigate(
      routes.apiPortalPublish(orgHandle, projectHandler, apiHandler, publication.apiPortalId),
      // Hands the portal's display name down so PortalPublishPage can title
      // itself without a second fetch just to look up a name it was already
      // shown here.
      { state: { portalName: publication.apiPortalName } },
    );
  };

  // `isPending`, not `isLoading`: the query is gated on the org and API handle
  // resolving, and a disabled query reports `isLoading: false` with no data —
  // which would flash the empty state while scope is still resolving.
  if (publicationsQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (publicationsQuery.error) {
    return <ErrorState message={intl.formatMessage(messages.errorMessage)} />;
  }

  const publications = publicationsQuery.data?.list ?? [];

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} values={{ apiName: component?.displayName ?? '' }} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
      </PageTitle>

      {publications.length === 0 ? (
        <EmptyState
          actionLabel={intl.formatMessage(messages.emptyAction)}
          description={intl.formatMessage(messages.emptyDescription)}
          onAction={() => navigate(routes.organizationPortals(orgHandle))}
          title={intl.formatMessage(messages.emptyTitle)}
        />
      ) : (
        <Box
          sx={{
            display: 'grid',
            gap: 2,
            gridTemplateColumns: {
              xs: '1fr',
              sm: 'repeat(2, 1fr)',
              md: 'repeat(3, 1fr)',
            },
            // Allow cards to shrink so long text does not widen the grid.
            '& > *': { minWidth: 0 },
          }}
        >
          {publications.map((publication) => (
            <PortalPublicationCard
              key={publication.apiPortalId}
              onOpen={openPublication}
              publication={publication}
            />
          ))}
        </Box>
      )}
    </>
  );
}
