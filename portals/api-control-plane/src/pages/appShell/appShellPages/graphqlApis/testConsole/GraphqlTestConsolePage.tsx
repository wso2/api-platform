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

import { Box, Card, PageTitle, Stack } from '@wso2/oxygen-ui';
import { Rocket } from '@wso2/oxygen-ui-icons-react';
import { createGraphiQLFetcher, createLocalStorage, type Fetcher } from '@graphiql/toolkit';
import { GraphiQL } from 'graphiql';
import 'graphiql/setup-workers/vite';
import 'graphiql/style.css';
import { useMemo, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate, useParams } from 'react-router-dom';

import { AppPage } from '@/components/AppPage';
import { useGraphQLApi, useGraphQLApiSdl } from '@/api/resources/graphqlApis';
import { useGraphQLApiGateways } from '@/api/resources/graphqlApis/apiGateways';
import { useDeployments } from '@/api/resources/graphqlApis/deployments';
import { isApiError } from '@/api/core/errors';
import { CSRF_HEADER, CSRF_HEADER_VALUE } from '@/contexts/auth/authConstants';
import { GatewayIllustration } from '@/components/illustrations/GatewayIllustration';
import { useIsDarkScheme } from '@/hooks/useIsDarkScheme';
import { useNow } from '@/hooks/useNow';
import { useNotifications } from '@/components/Notifications';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { routes } from '@/routes/paths';
import { buildInvokeUrl, gatewayEndpoint } from '../../gateways/utils/gatewayDisplay';
import { parseGraphQLSdl } from '../../apis/create/utils/graphqlSchema';
import { GatewaySection } from '../../test/components/GatewaySection';
import { TestKeySection } from '../../test/components/TestKeySection';
import { apiKeyAuthOf } from '../../test/utils/apiKeyAuth';
import { deployedGateways as deployedGatewaysOf } from '../../test/utils/deployedGateways';
import { gatewayOptionLabel } from '../../test/utils/gatewayOptionLabel';
import {
  isTestKeyExpired,
  KEY_COUNTDOWN_TICK_MS,
  testKeyRemainingMs,
} from '../../test/utils/testApiKey';
import { useGraphQLTestApiKey } from '../../test/utils/useTestApiKey';
import { GraphiqlThemeStyles } from './GraphiqlThemeStyles';
import { withValidOperationName } from './withValidOperationName';

// The REST test page's messages are reused by ID where the copy is identical,
// so the two pages read the same and share their translations.
const messages = defineMessages({
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.testConsole.GraphqlTestConsolePage.apiNotFound',
    defaultMessage: 'GraphQL API not found',
  },
  deployAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.deployAction',
    defaultMessage: 'Deploy API',
    description: 'Button in the not-deployed banner that opens the Deploy page. A command.',
  },
  definitionUnavailable: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.definitionUnavailable',
    defaultMessage: 'This API’s definition could not be loaded.',
    description:
      "Shown when fetching or parsing the API's OpenAPI definition failed — distinct from the API simply not having one.",
  },
  deploymentUnavailable: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.deploymentUnavailable',
    defaultMessage: 'This API’s deployments could not be loaded.',
    description:
      'Shown when fetching the gateways or deployments failed — distinct from the API genuinely not being deployed anywhere.',
  },
  fetchFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.testConsole.GraphqlTestConsolePage.fetchFailed',
    defaultMessage:
      'Could not reach the test console service. Check your connection and try again.',
    description:
      'Shown when the browser fetch to this app\'s own backend fails below the HTTP level (network error) — a bare "Failed to fetch" the browser gives no further detail on. The request never leaves this origin: it is proxied server-side to the API\'s gateway, so this is not a CORS or gateway-certificate problem.',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.loading',
    defaultMessage: 'Loading test console',
    description: 'Shown while the API and its definition are being fetched.',
  },
  notDeployedTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.notDeployedTitle',
    defaultMessage: 'You must deploy the API to start testing.',
    description:
      'Heading of the empty state shown when the API is not deployed anywhere, so there is no endpoint to send requests to.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.testConsole.GraphqlTestConsolePage.subtitle',
    defaultMessage: 'Send queries and mutations to a deployed gateway.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.test.TestPage.title',
    defaultMessage: 'Test',
    description: 'Page heading.',
  },
});

const GRAPHIQL_HEIGHT = 'calc(100vh - 380px)';
const GRAPHIQL_MIN_HEIGHT = 560;

/**
 * GraphiQL derives a tab's title from its operation name — `operationName ||
 * fuzzyExtractOperationName(query) || '<untitled>'` (`@graphiql/react`'s
 * `tabs.ts`); there is no separate "tab title" prop, and that extractor
 * ignores every line starting with `#`. So GraphiQL's own built-in welcome
 * comment (reproduced below, verbatim except for the keyboard-shortcuts
 * section — those lines interpolate an OS-aware Cmd/Ctrl label via a helper
 * this project doesn't depend on directly) is kept as-is, with a real,
 * non-comment `query Schema { ... }` appended after it: the comment lines
 * stay invisible to the title extractor, so the tab still reads "Schema"
 * instead of GraphiQL's own default "<untitled>", while the user still sees
 * the original explanation and example on first load. `__typename` is a
 * placeholder field valid against any schema, so this loads with no
 * syntax/validation error before the user replaces it with a real query.
 */
const DEFAULT_QUERY = `# Welcome to GraphiQL
#
# GraphiQL is an in-browser tool for writing, validating, and testing
# GraphQL queries.
#
# Type queries into this side of the screen, and you will see intelligent
# typeaheads aware of the current GraphQL type schema and live syntax and
# validation errors highlighted within the text.
#
# GraphQL queries typically start with a "{" character. Lines that start
# with a # are ignored.
#
# An example GraphQL query might look like:
#
#     {
#       field(arg: "value") {
#         subField
#       }
#     }
#

query Schema {
  __typename
}
`;

/**
 * GraphQL Test console: the REST test page's layout — heading, gateway card,
 * test key and not-deployed state — with GraphiQL in place of the cURL
 * builder. Built on the real `graphiql`/`@graphiql/toolkit` packages rather
 * than a bespoke editor, matching the console devportal embeds for the same
 * APIs. No `ScopeGate`: this route lives outside `ConsoleScopeProvider`'s
 * REST-only api-scope matching (see `graphqlApiPath`), so it guards on its
 * own route param instead.
 *
 * The schema is passed in directly from the API's stored SDL (parsed with
 * the same `parseGraphQLSdl` the wizard/overview pages use) so GraphiQL
 * skips introspection — which would otherwise require the gateway's own
 * auth before the docs/autocomplete could load at all.
 */
export function GraphqlTestConsolePage() {
  return (
    <AppPage>
      <GraphqlTestConsolePageContent />
    </AppPage>
  );
}

function GraphqlTestConsolePageContent() {
  const intl = useIntl();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const { graphqlApiHandler, orgHandle, projectHandler } = useParams();
  const isDark = useIsDarkScheme();

  const apiQuery = useGraphQLApi(graphqlApiHandler);
  const sdlQuery = useGraphQLApiSdl(graphqlApiHandler);
  // The API's own gateway associations, as the REST test page reads them —
  // scoped to the API, so this needs no organization-wide gateway access.
  const gatewaysQuery = useGraphQLApiGateways(graphqlApiHandler);
  const deploymentsQuery = useDeployments(graphqlApiHandler);

  /** API `api-key-auth` configuration, if present. */
  const apiKeyAuth = useMemo(() => apiKeyAuthOf(apiQuery.data), [apiQuery.data]);
  const needsApiKey = apiKeyAuth !== undefined;

  const gateways = useMemo(
    () => deployedGatewaysOf(gatewaysQuery.data?.list ?? [], deploymentsQuery.data?.list ?? []),
    [deploymentsQuery.data, gatewaysQuery.data],
  );

  /** True while we still do not know whether the API is deployed. */
  const deploymentUnknown = gatewaysQuery.isPending || deploymentsQuery.isPending;
  /** True when either deployment lookup failed. */
  const deploymentFailed = Boolean(gatewaysQuery.error || deploymentsQuery.error);
  /** True when both deployment lookups settled and no gateway has the API. */
  const notDeployed = !deploymentUnknown && !deploymentFailed && gateways.length === 0;
  /** True when a request can be sent. */
  const deploymentReady = !deploymentUnknown && !deploymentFailed && gateways.length > 0;

  /** Mint only when a required key has a gateway to use. */
  const testApiKey = useGraphQLTestApiKey(graphqlApiHandler, needsApiKey && deploymentReady);
  const testKey = testApiKey.key;
  /** State-driven tick for the key countdown and expiry. */
  const now = useNow(testKey ? KEY_COUNTDOWN_TICK_MS : 0);
  const keyName = apiKeyAuth?.name ?? '';
  const credentialIn = apiKeyAuth?.in ?? 'header';
  /** The credential sent with every request, once a live key exists. */
  const credential =
    needsApiKey && testKey && !isTestKeyExpired(testKey, now) && keyName.trim() !== ''
      ? { name: keyName, value: testKey.value }
      : undefined;

  const [selectedGatewayId, setSelectedGatewayId] = useState('');
  const selectedGateway =
    gateways.find((gateway) => gateway.id === selectedGatewayId) ?? gateways[0];

  /**
   * True when the API has no stored SDL (`404`). Unlike REST's missing
   * definition this does not block testing: GraphiQL falls back to
   * introspecting the endpoint for its schema.
   */
  const sdlMissing = !sdlQuery.isPending && isApiError(sdlQuery.error) && sdlQuery.error.isNotFound;

  const schema = useMemo(() => {
    if (!sdlQuery.data) return undefined;
    const parsed = parseGraphQLSdl(sdlQuery.data);
    return 'schema' in parsed ? parsed.schema : undefined;
  }, [sdlQuery.data]);

  const storage = useMemo(
    // The version suffix isn't a real schema version — it's a cache-buster.
    // GraphiQL restores a previously-persisted tab verbatim once one exists
    // under a namespace (`getDefaultTabState` in `@graphiql/react`'s tabs.ts
    // only ever falls back to `defaultQuery` when storage has nothing stored
    // yet), so a browser that already opened this console under an older
    // namespace would keep showing that stale tab forever regardless of what
    // `defaultQuery` is set to now — this is why the suffix was bumped from
    // v1 to v2 (to stop showing GraphiQL's own default "<untitled>" tab) and
    // now to v3 (to pick up DEFAULT_QUERY's added welcome/example comment).
    // Bump it again any time DEFAULT_QUERY's content changes, or affected
    // browsers keep seeing whatever they had cached before this exact fix.
    () => createLocalStorage({ namespace: `wso2-graphql-console:v3:${graphqlApiHandler}` }),
    [graphqlApiHandler],
  );

  const api = apiQuery.data;
  // The real gateway URL — shown/copyable so it can be called directly (e.g.
  // from curl), but not what the browser talks to; see invokeProxyUrl.
  const endpointUrl =
    selectedGateway && api
      ? buildInvokeUrl(gatewayEndpoint(selectedGateway), api.context, api.version)
      : '';
  // The browser never calls the gateway directly — it would need the
  // gateway's own CORS policy configured and, for a dev/self-signed
  // certificate, to be trusted first. Instead this same-origin BFF route
  // resolves and proxies to the real gateway endpoint server-side (see
  // bff/internal/server/graphql_invoke.go), so the query/variables/headers
  // the user enters reach the gateway exactly as typed while the browser
  // request itself stays same-origin. A query-parameter credential rides on
  // this URL; the BFF forwards the query string to the gateway unchanged.
  const invokeProxyUrl = useMemo(() => {
    if (!graphqlApiHandler || !selectedGateway?.id) return '';
    const base = `/api/graphql-console/${encodeURIComponent(graphqlApiHandler)}/gateways/${encodeURIComponent(selectedGateway.id)}/invoke`;
    if (!credential || credentialIn !== 'query') return base;
    return `${base}?${new URLSearchParams({ [credential.name]: credential.value }).toString()}`;
  }, [credential, credentialIn, graphqlApiHandler, selectedGateway?.id]);
  // Supplies a wrapped `fetch` to createGraphiQLFetcher rather than wrapping
  // the fetcher's own return value: the toolkit doesn't always resolve a
  // plain Promise<ExecutionResult> (it can wrap even a non-subscription
  // response as an async-iterable for incremental-delivery support), so a
  // try/catch around calling the fetcher never sees the underlying fetch()
  // rejection. `fetch` itself is the one place a network-level failure (no
  // HTTP response at all) surfaces as a plain rejected promise, so
  // intercepting there is what actually works. Since the request target is
  // this app's own same-origin BFF route, such a failure means the BFF
  // itself couldn't be reached — not a CORS or gateway-certificate issue,
  // both of which this proxy already sidesteps. An HTTP error response
  // (4xx/5xx, including a GraphQL "errors" body, or the BFF's own structured
  // error for a resolution failure) resolves normally and is untouched here —
  // GraphiQL's own response panel keeps showing those as it always has. The
  // error is rethrown either way so GraphiQL's panel still reflects the
  // failure; this only adds the toast alongside it.
  const guardedFetch = useMemo<typeof fetch>(
    () =>
      async (...args) => {
        try {
          return await fetch(...args);
        } catch (error) {
          if (error instanceof TypeError) {
            notify(intl.formatMessage(messages.fetchFailed), 'error');
          }
          throw error;
        }
      },
    [notify, intl],
  );
  // A header credential is a base header: GraphiQL's own Headers tab is
  // applied after it, so a value typed there still overrides the test key.
  const credentialHeaderName = credential && credentialIn === 'header' ? credential.name : '';
  const credentialHeaderValue = credential && credentialIn === 'header' ? credential.value : '';
  const fetcher: Fetcher = useMemo(() => {
    const baseFetcher = createGraphiQLFetcher({
      fetch: guardedFetch,
      headers: {
        [CSRF_HEADER]: CSRF_HEADER_VALUE,
        ...(credentialHeaderName ? { [credentialHeaderName]: credentialHeaderValue } : {}),
      },
      url: invokeProxyUrl,
    });
    return (params, opts) => baseFetcher(withValidOperationName(params), opts);
  }, [credentialHeaderName, credentialHeaderValue, guardedFetch, invokeProxyUrl]);

  if (!graphqlApiHandler || apiQuery.error) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }
  // Wait for deployment state too, so the page never flashes the console
  // before knowing there is an endpoint to send to.
  if (apiQuery.isPending || deploymentUnknown) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (!api) return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;

  /** Shared by the empty states and the page itself, so they cannot drift. */
  const heading = (
    <PageTitle.Header>
      <Stack alignItems="baseline" direction="row" spacing={1.5}>
        <FormattedMessage {...messages.title} /> {api.displayName}
      </Stack>
    </PageTitle.Header>
  );

  if (deploymentFailed) {
    return (
      <>
        <PageTitle>{heading}</PageTitle>
        <ErrorState title={intl.formatMessage(messages.deploymentUnavailable)} />
      </>
    );
  }

  if (notDeployed) {
    return (
      <>
        <PageTitle>{heading}</PageTitle>
        <EmptyState
          actionIcon={<Rocket size={18} />}
          actionLabel={intl.formatMessage(messages.deployAction)}
          illustration={<GatewayIllustration />}
          onAction={() =>
            navigate(
              routes.graphqlApiDeploy(orgHandle ?? '', projectHandler ?? null, graphqlApiHandler),
            )
          }
          title={intl.formatMessage(messages.notDeployedTitle)}
        />
      </>
    );
  }

  // Wait for the schema, as REST waits for its definition: rendering GraphiQL
  // without one makes it introspect the endpoint in the meantime. Checked
  // after deployment so an undeployed API needs no schema yet.
  if (sdlQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (sdlQuery.error && !sdlMissing) {
    return (
      <>
        <PageTitle>{heading}</PageTitle>
        <ErrorState title={intl.formatMessage(messages.definitionUnavailable)} />
      </>
    );
  }

  return (
    <>
      <PageTitle>
        {heading}
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
      </PageTitle>

      <Stack spacing={2}>
        <Card variant="outlined">
          <GatewaySection
            endpoint={endpointUrl}
            gateways={gateways}
            onSelect={setSelectedGatewayId}
            optionLabel={gatewayOptionLabel}
            selectedGatewayId={selectedGateway?.id ?? ''}
          />
          {needsApiKey && (
            <TestKeySection
              error={Boolean(testApiKey.error)}
              keyName={keyName}
              loading={testApiKey.isPending}
              location={credentialIn}
              onRegenerate={testApiKey.regenerate}
              regenerating={testApiKey.isRegenerating}
              remainingMs={testKeyRemainingMs(testKey, now)}
              value={testKey?.value}
            />
          )}
        </Card>

        <Box
          sx={{
            border: '1px solid',
            borderColor: 'divider',
            height: GRAPHIQL_HEIGHT,
            minHeight: GRAPHIQL_MIN_HEIGHT,
            overflow: 'hidden',
          }}
        >
          {/*
            No `shouldPersistHeaders`: GraphiQL's own default (false) strips
            the Headers tab out of the tab state it writes to `storage`
            (@graphiql/react's serializeTabState). `storage` above is real
            `localStorage` — persisting headers would write whatever
            Authorization/API-key value the user typed there to disk in
            cleartext. Do not re-add this prop; the query/variables tabs are
            still persisted either way, only headers are excluded.
          */}
          {/* GraphiQL in the console's own colours and font, following its
              light/dark scheme rather than the operating system's. */}
          <GraphiqlThemeStyles />
          <GraphiQL
            defaultQuery={DEFAULT_QUERY}
            fetcher={fetcher}
            forcedTheme={isDark ? 'dark' : 'light'}
            schema={schema}
            storage={storage}
          />
        </Box>
      </Stack>
    </>
  );
}
