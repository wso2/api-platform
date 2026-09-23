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

import { PageTitle } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useParams } from 'react-router-dom';

import { useGraphQLApi } from '@/api/resources/graphqlApis';
import { ComingSoon } from '@/components/ComingSoon';
import { ErrorState, LoadingState } from '@/components/StateViews';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.ApiPortalPublicationsList.title',
    defaultMessage: 'Publish {apiName}',
    description: 'Page heading. {apiName} is the API display name, user-supplied; do not translate it.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.publish.GraphqlPublishPage.subtitle',
    defaultMessage: 'List this API in the Developer Portal for consumers to discover and subscribe to.',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.loading',
    defaultMessage: 'Loading API',
    description: 'Shown while the API being edited is fetched.',
  },
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.publish.GraphqlPublishPage.apiNotFound',
    defaultMessage: 'GraphQL API not found',
  },
  feature: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.publish.GraphqlPublishPage.feature',
    defaultMessage: 'Publishing this API to the Developer Portal',
  },
});

/**
 * Fork of `manage/LifeCyclePage.tsx` ("Publish to Devportal") for a GraphQL
 * API. REST's own stepper step is itself just a `ComingSoon` placeholder
 * today — `GraphQLAPIDetail` (like `RESTAPI`, in this respect) has no
 * lifecycle-status field to drive a real publish flow, and the actual
 * dev-portal listing lives entirely in the separate, unwired `api-portal`
 * service (see `ProjectStatistics.tsx`'s comment on the same gap). This
 * mirrors REST's own current (non-functional) parity, not a new backend
 * capability. No `ScopeGate`: this route lives outside `ConsoleScopeProvider`'s
 * REST-only api-scope matching (see `graphqlApiPath`), so it guards on its own
 * route param instead.
 */
export function GraphqlPublishPage() {
  const intl = useIntl();
  const { graphqlApiHandler } = useParams();
  const apiQuery = useGraphQLApi(graphqlApiHandler);

  if (!graphqlApiHandler || apiQuery.error) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }
  if (apiQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (!apiQuery.data) return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;

  const api = apiQuery.data;

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} values={{ apiName: api.displayName }} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
      </PageTitle>
      <ComingSoon feature={<FormattedMessage {...messages.feature} />} />
    </>
  );
}
