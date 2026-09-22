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
  Alert,
  Box,
  Chip,
  FormControl,
  IconButton,
  InputAdornment,
  MenuItem,
  Select,
  Stack,
  TextField,
  Tooltip,
} from '@wso2/oxygen-ui';
import { Copy } from '@wso2/oxygen-ui-icons-react';
import { createGraphiQLFetcher, createLocalStorage } from '@graphiql/toolkit';
import { GraphiQL } from 'graphiql';
import 'graphiql/setup-workers/vite';
import 'graphiql/style.css';
import { useMemo, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useParams } from 'react-router-dom';

import { useGateways, type Gateway } from '@/api/resources/gateways';
import { useGraphQLApi, useGraphQLApiSdl } from '@/api/resources/graphqlApis';
import { useDeployments } from '@/api/resources/graphqlApis/deployments';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { versionLabel } from '@/utils/versionLabel';
import { buildInvokeUrl } from '../../apis/overview/InvokeUrlPanel';
import { gatewayEndpoint } from '../../gateways/utils/gatewayDisplay';
import { parseGraphQLSdl } from '../../apis/create/utils/graphqlSchema';

const messages = defineMessages({
  apiNotFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.testConsole.GraphqlTestConsolePage.apiNotFound',
    defaultMessage: 'GraphQL API not found',
  },
  copyEndpoint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.testConsole.GraphqlTestConsolePage.copyEndpoint',
    defaultMessage: 'Copy endpoint URL',
  },
  copyEndpointFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.testConsole.GraphqlTestConsolePage.copyEndpointFailed',
    defaultMessage: 'Failed to copy the endpoint URL.',
  },
  copyEndpointSucceeded: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.testConsole.GraphqlTestConsolePage.copyEndpointSucceeded',
    defaultMessage: 'Endpoint URL copied to clipboard.',
  },
  endpointLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.testConsole.GraphqlTestConsolePage.endpointLabel',
    defaultMessage: 'Endpoint URL',
    description: 'Accessible name of the read-only field holding the gateway invoke URL under test.',
  },
  gatewayLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.testConsole.GraphqlTestConsolePage.gatewayLabel',
    defaultMessage: 'Gateway',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.testConsole.GraphqlTestConsolePage.loading',
    defaultMessage: 'Loading test console',
  },
  noDeployment: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.testConsole.GraphqlTestConsolePage.noDeployment',
    defaultMessage: 'Deploy this API to a gateway before testing it here.',
  },
});

const GRAPHIQL_HEIGHT = 'calc(100vh - 280px)';
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
 * GraphQL Test console, built on the real `graphiql`/`@graphiql/toolkit`
 * packages rather than a bespoke editor — matches the console devportal
 * embeds for the same APIs. No `ScopeGate`: this route lives outside
 * `ConsoleScopeProvider`'s REST-only api-scope matching (see
 * `graphqlApiPath`), so it guards on its own route param instead.
 *
 * The schema is passed in directly from the API's stored SDL (parsed with
 * the same `parseGraphQLSdl` the wizard/overview pages use) so GraphiQL
 * skips introspection — which would otherwise require the gateway's own
 * auth before the docs/autocomplete could load at all.
 */
export function GraphqlTestConsolePage() {
  const intl = useIntl();
  const { notify } = useNotifications();
  const { graphqlApiHandler } = useParams();

  const apiQuery = useGraphQLApi(graphqlApiHandler);
  const sdlQuery = useGraphQLApiSdl(graphqlApiHandler);
  const gatewaysQuery = useGateways();
  const deploymentsQuery = useDeployments(graphqlApiHandler);

  const deployedGateways = useMemo((): Gateway[] => {
    const gateways = gatewaysQuery.data?.list ?? [];
    const deployedIds = new Set(
      (deploymentsQuery.data?.list ?? [])
        .filter((deployment) => deployment.status === 'DEPLOYED')
        .map((deployment) => deployment.gatewayId),
    );
    return gateways.filter((gateway) => deployedIds.has(gateway.id ?? ''));
  }, [gatewaysQuery.data, deploymentsQuery.data]);

  const [selectedGatewayId, setSelectedGatewayId] = useState('');

  const selectedGateway =
    deployedGateways.find((gateway) => gateway.id === selectedGatewayId) ?? deployedGateways[0];

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
  const endpointUrl =
    selectedGateway && api
      ? buildInvokeUrl(gatewayEndpoint(selectedGateway), api.context, api.version)
      : '';
  const fetcher = useMemo(() => createGraphiQLFetcher({ url: endpointUrl }), [endpointUrl]);

  if (!graphqlApiHandler || apiQuery.error) {
    return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;
  }
  if (apiQuery.isPending || gatewaysQuery.isPending || deploymentsQuery.isPending) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (!api) return <ErrorState title={intl.formatMessage(messages.apiNotFound)} />;

  const copyEndpoint = () => {
    if (!endpointUrl) return;
    navigator.clipboard
      ?.writeText(endpointUrl)
      .then(() => notify(intl.formatMessage(messages.copyEndpointSucceeded), 'success'))
      .catch(() => notify(intl.formatMessage(messages.copyEndpointFailed), 'error'));
  };

  return (
    <>
      {deployedGateways.length === 0 ? (
        <Alert severity="info" sx={{ mb: 3 }}>
          <FormattedMessage {...messages.noDeployment} />
        </Alert>
      ) : (
        <>
          <Stack alignItems="center" direction="row" spacing={1.5} sx={{ mb: 2 }}>
            <Chip label={versionLabel(api.version)} size="small" variant="outlined" />
            <FormControl size="small" sx={{ minWidth: 220 }}>
              <Select
                inputProps={{ 'aria-label': intl.formatMessage(messages.gatewayLabel) }}
                onChange={(event) => setSelectedGatewayId(String(event.target.value))}
                value={selectedGateway?.id || ''}
              >
                {deployedGateways.map((gateway) => (
                  <MenuItem key={gateway.id} value={gateway.id ?? ''}>
                    {gateway.displayName}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <TextField
              fullWidth
              size="small"
              slotProps={{
                htmlInput: {
                  'aria-label': intl.formatMessage(messages.endpointLabel),
                  sx: { fontFamily: 'monospace' },
                },
                input: {
                  readOnly: true,
                  endAdornment: (
                    <InputAdornment position="end">
                      <Tooltip title={intl.formatMessage(messages.copyEndpoint)}>
                        <span>
                          <IconButton
                            aria-label={intl.formatMessage(messages.copyEndpoint)}
                            disabled={!endpointUrl}
                            onClick={copyEndpoint}
                            size="small"
                          >
                            <Copy size={16} />
                          </IconButton>
                        </span>
                      </Tooltip>
                    </InputAdornment>
                  ),
                },
              }}
              sx={{ flex: 1 }}
              value={endpointUrl}
            />
          </Stack>

          <Box
            sx={{
              border: '1px solid',
              borderColor: 'divider',
              height: GRAPHIQL_HEIGHT,
              minHeight: GRAPHIQL_MIN_HEIGHT,
              overflow: 'hidden',
            }}
          >
            <GraphiQL
              defaultQuery={DEFAULT_QUERY}
              fetcher={fetcher}
              schema={schema}
              shouldPersistHeaders
              storage={storage}
            />
          </Box>
        </>
      )}
    </>
  );
}
