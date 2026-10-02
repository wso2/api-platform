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
import { Link, useNavigate, useParams } from 'react-router-dom';

import {
  useGraphQLApi,
  useGraphQLApiSdl,
  useUpdateGraphQLApi,
  type GraphQLApiDetail,
} from '@/api/resources/graphqlApis';
import { AppPage } from '@/components/AppPage';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { resuppliedSchemaSource } from '../utils/graphqlApiMetadataUpdate';
import { EditGraphqlApiForm, type GraphqlApiBasicInfoFormValues } from './EditGraphqlApiForm';

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
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.edit.GraphqlApiEditPage.notFound',
    defaultMessage: 'GraphQL API not found',
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
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.edit.GraphqlApiEditPage.subtitle',
    defaultMessage: 'Change the name, description, context, version and endpoint of this API.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.ApiEditPage.title',
    defaultMessage: 'Edit API',
  },
});

/**
 * Applies the form's four fields to the fetched API.
 *
 * The update endpoint's body is the whole `GraphQLAPI` (packed into the
 * multipart envelope `updateGraphQLApi` expects), so the original is spread
 * back with the edits laid over it — anything the form does not collect
 * (policies, subscriptionPlans, upstream.sandbox, upstream.main.ref/auth) has
 * to survive the round trip untouched, same as `ApiEditPage`'s
 * `toUpdateBody`. `upstream.main.url` is the one upstream field the form does
 * collect (see `EditGraphqlApiForm`'s `hasUrlUpstream`) — merged onto the
 * existing `main`, never replacing it outright, so `auth`/`hostRewrite` on a
 * URL-based upstream survive an endpoint-only edit.
 *
 * This is a metadata-only edit — the schema itself is untouched — so
 * `schemaSource`/`sdl` are resupplied via `resuppliedSchemaSource` rather
 * than forced to `'introspection'` (see that function's doc comment).
 */
const toUpdateBody = (
  api: GraphQLApiDetail,
  sdl: string,
  values: GraphqlApiBasicInfoFormValues,
) => ({
  metadata: {
    ...api,
    context: values.context,
    description: values.description,
    displayName: values.displayName,
    ...resuppliedSchemaSource(api, sdl),
    // Only a URL-based main upstream is ever form-editable (see
    // EditGraphqlApiForm's hasUrlUpstream) — a ref-based one has no
    // endpointUrl draft to apply, so upstream survives untouched for it.
    ...(api.upstream?.main?.url !== undefined
      ? {
          upstream: {
            ...api.upstream,
            main: { ...api.upstream.main, url: values.endpointUrl.trim() },
          },
        }
      : {}),
    version: values.version.trim(),
  },
});

// No `ScopeGate`: this route lives outside `ConsoleScopeProvider`'s REST-only
// api-scope matching (see `graphqlApiPath`), so it guards on its own route
// param instead — and is only reachable from the detail page's own edit
// button, so it never mounts without an API in scope.
export function GraphqlApiEditPage() {
  return (
    <AppPage>
      <GraphqlApiEditPageContent />
    </AppPage>
  );
}

function GraphqlApiEditPageContent() {
  const intl = useIntl();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const { params } = useConsoleScope();
  const { graphqlApiHandler } = useParams();

  const apiQuery = useGraphQLApi(graphqlApiHandler);
  // Needed to faithfully resupply a non-introspection schemaSource on save —
  // see toUpdateBody. Fetched unconditionally since schemaSource isn't known
  // until apiQuery resolves.
  const sdlQuery = useGraphQLApiSdl(graphqlApiHandler);
  const updateApi = useUpdateGraphQLApi();

  const detailPath = routes.graphqlApi(
    params.orgHandle ?? '',
    params.projectHandler ?? '',
    graphqlApiHandler ?? '',
  );

  if (!graphqlApiHandler || apiQuery.error || sdlQuery.error) {
    return <ErrorState title={intl.formatMessage(messages.notFound)} />;
  }
  if (apiQuery.isPending || sdlQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (!apiQuery.data) {
    return <ErrorState title={intl.formatMessage(messages.notFound)} />;
  }

  const api = apiQuery.data;
  const sdl = sdlQuery.data ?? '';

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

  const save = (values: GraphqlApiBasicInfoFormValues) => {
    updateApi.mutate(
      { graphqlApiId: graphqlApiHandler, body: toUpdateBody(api, sdl, values) },
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
        <PageTitle.BackButton
          component={<Link to={detailPath} />}
          sx={{
            alignItems: 'center',
            display: 'flex',
            marginLeft: '-10px',
            textDecoration: 'none',
          }}
        >
          <FormattedMessage {...messages.back} />
        </PageTitle.BackButton>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader variant="caption">
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
      </PageTitle>

      <EditGraphqlApiForm
        api={api}
        fieldErrors={updateApi.error?.fieldErrorMap()}
        isSaving={updateApi.isPending}
        onCancel={() => void navigate(detailPath)}
        onSubmit={save}
      />
    </>
  );
}
