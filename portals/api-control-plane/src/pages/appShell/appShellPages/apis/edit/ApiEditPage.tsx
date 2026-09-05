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
import { Link, useNavigate } from 'react-router-dom';

import { useRestApi, useUpdateRestApi, type RestApi } from '@/api/resources/restApis';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { EditApiForm, type ApiBasicInfoFormValues } from './components/EditApiForm';

const messages = defineMessages({
  back: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.back',
    defaultMessage: 'Back to API',
    description: 'Back button above the edit form, returning to the API overview.',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.loading',
    defaultMessage: 'Loading API',
    description: 'Shown while the API being edited is fetched.',
  },
  notFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.notFound',
    defaultMessage: 'API not found',
    description: 'Shown when the API to edit could not be loaded.',
  },
  readOnly: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.readOnly',
    defaultMessage: 'This API cannot be edited here',
    description: 'Title shown when the edit page is opened for a gateway-managed API.',
  },
  readOnlyDetail: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.readOnlyDetail',
    defaultMessage:
      'It was discovered from a data-plane gateway, so it is read-only in this console.',
    description: 'Explains why a gateway-managed API cannot be edited.',
  },
  saved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.saved',
    defaultMessage: 'API updated.',
    description: 'Confirmation shown after the API details are saved.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.subtitle',
    defaultMessage: 'Change the name, description, context, version and backend of this API.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.title',
    defaultMessage: 'Edit API',
  },
});

/**
 * Applies the form's five fields to the fetched API.
 *
 * The spec's update body is the whole `RESTAPI`, so the original is spread back
 * with the edits laid over it — anything the form does not collect (operations,
 * policies, transports) has to survive the round trip untouched.
 *
 * `upstream` is only rewritten when the API routes to a direct URL. An API
 * pointing at a shared upstream `ref` keeps it: `url` and `ref` are mutually
 * exclusive, so writing both would be rejected.
 */
const toUpdateBody = (api: RestApi, values: ApiBasicInfoFormValues): RestApi => ({
  ...api,
  context: values.context,
  description: values.description,
  displayName: values.displayName,
  version: values.version,
  upstream: api.upstream?.main?.ref
    ? api.upstream
    : {
        ...api.upstream,
        main: { ...api.upstream?.main, url: values.targetUrl },
      },
});

// No `ScopeGate`: this page is only reachable from the API detail page's own
// edit button, so it never mounts without an API in scope.
export function ApiEditPage() {
  const intl = useIntl();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const { params } = useConsoleScope();

  const apiQuery = useRestApi(params.apiHandler);
  const updateApi = useUpdateRestApi();

  const detailPath = routes.api(
    params.orgHandle ?? '',
    params.projectHandler ?? '',
    params.apiHandler ?? '',
  );

  if (apiQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (apiQuery.error || !apiQuery.data) {
    return <ErrorState title={intl.formatMessage(messages.notFound)} />;
  }

  const api = apiQuery.data;
  const restApiId = api.id ?? params.apiHandler ?? '';

  // The detail page hides the edit button for a gateway-managed API; this is
  // the same rule enforced at the page, for anyone arriving by URL.
  if (api.readOnly) {
    return (
      <ErrorState
        message={intl.formatMessage(messages.readOnlyDetail)}
        title={intl.formatMessage(messages.readOnly)}
      />
    );
  }

  const save = (values: ApiBasicInfoFormValues) => {
    updateApi.mutate(
      { restApiId, body: toUpdateBody(api, values) },
      {
        onSuccess: () => {
          notify(intl.formatMessage(messages.saved), 'success');
          void navigate(detailPath);
        },
      },
    );
  };

  return (
    <>
      <PageTitle>
        <Link to={detailPath}>
          <PageTitle.BackButton>
            <FormattedMessage {...messages.back} />
          </PageTitle.BackButton>
        </Link>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
      </PageTitle>

      <EditApiForm
        api={api}
        fieldErrors={updateApi.error?.fieldErrorMap()}
        isSaving={updateApi.isPending}
        onCancel={() => void navigate(detailPath)}
        onSubmit={save}
      />
    </>
  );
}
