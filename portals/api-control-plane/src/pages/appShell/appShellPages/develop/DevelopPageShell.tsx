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

import type { ReactNode } from 'react';
import { PageTitle } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { useRestApi, type RestApi } from '@/api/resources/restApis';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';

const messages = defineMessages({
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.DevelopPageShell.loading',
    defaultMessage: 'Loading API',
    description: 'Shown while the API being edited is fetched.',
  },
  notFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.DevelopPageShell.notFound',
    defaultMessage: 'API not found',
  },
});

export type DevelopPageShellProps = {
  /** Section name, e.g. Policies. */
  title: MessageDescriptor;
  /** Sub-header taking the API's `displayName` as `{apiName}`. */
  subtitle: MessageDescriptor;
  children: (api: RestApi) => ReactNode;
};

/**
 * Heading plus loaded API detail for a Develop page.
 *
 * The three panels were tabs on the API overview page, where one query served
 * all of them and the header card said which API you were looking at. Now that
 * each is its own route they each need both, so this holds the pair in one
 * place rather than repeating the query and its loading/error branches three
 * times. `children` takes the loaded API, so a panel needing it can't be
 * rendered before it exists.
 */
export function DevelopPageShell({ children, subtitle, title }: DevelopPageShellProps) {
  const intl = useIntl();
  const { params } = useConsoleScope();
  const apiQuery = useRestApi(params.apiHandler);

  if (apiQuery.isPending) return <LoadingState label={intl.formatMessage(messages.loading)} />;
  if (apiQuery.error || !apiQuery.data) {
    return <ErrorState title={intl.formatMessage(messages.notFound)} />;
  }

  const api = apiQuery.data;

  return (
    <>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...subtitle} values={{ apiName: api.displayName }} />
        </PageTitle.SubHeader>
      </PageTitle>

      {children(api)}
    </>
  );
}
