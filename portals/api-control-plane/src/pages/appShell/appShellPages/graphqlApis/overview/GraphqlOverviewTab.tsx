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

import { Box, Card, Grid, Stack } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage } from 'react-intl';
import { useParams } from 'react-router-dom';

import { GRAPHQL_API_TYPE } from '@/api/resources/apiPublications';
import type { Gateway } from '@/api/resources/gateways';
import { useUpdateGraphQLApi, type GraphQLApiDetail } from '@/api/resources/graphqlApis';
import type { Deployment } from '@/api/resources/graphqlApis/deployments';
import { routes } from '@/routes/paths';
import { GraphqlSchemaExplorer } from '../../apis/create/components/graphql/GraphqlSchemaExplorer';
import { ApiKeysPanel } from '../../apis/overview/ApiKeysPanel';
import { DeployedGatewaysPanel } from '../../apis/overview/DeployedGatewaysPanel';
import { DocumentsPanel } from '../../apis/overview/DocumentsPanel';
import { EndpointCard } from '../../apis/overview/EndpointsPanel';
import { InvokeUrlPanel } from '../../apis/overview/InvokeUrlPanel';
import { resuppliedSchemaSource } from '../utils/graphqlApiMetadataUpdate';

const messages = defineMessages({
  endpointTitle: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.endpoint',
    defaultMessage: 'Endpoint',
    description:
      'Label above the URL that requests from this console are sent to. Shown in capitals by the layout, so translate it as ordinary words.',
  },
});

/**
 * A GraphQL API's backend endpoint: REST's `EndpointCard` over GraphQL's own
 * update. A GraphQL API has exactly one endpoint, so the card keeps the
 * singular "Endpoint" title rather than REST's "Endpoints".
 *
 * The save differs from REST's because `UpdateGraphQLAPI`'s body is a
 * multipart `{ metadata, sdlFile }` envelope that must also resupply
 * `schemaSource`/`sdl` faithfully — see `resuppliedSchemaSource`'s doc
 * comment, and `GraphqlApiEditPage`, which applies the same rule.
 */
function EndpointPanel({ api, sdl }: { api: GraphQLApiDetail; sdl?: string }) {
  const updateApi = useUpdateGraphQLApi();
  const url = api.upstream?.main?.url;
  // Only a URL-based main upstream is ever editable here — a ref-based one
  // (`upstream.main.ref`) has no literal URL to edit, the same rule
  // `EditGraphqlApiForm`'s `hasUrlUpstream` applies.
  const hasUrlUpstream = url !== undefined;
  // sdl loads separately from api (see useGraphQLApiSdl) and is required to
  // faithfully resupply a non-introspection schemaSource on save; editing
  // before it's loaded risks silently saving an empty schema, so the edit
  // button waits for it — the same precondition `GraphqlApiEditPage` enforces
  // by blocking its whole page on both queries first.
  const canEdit = !api.readOnly && hasUrlUpstream && sdl !== undefined;

  return (
    <EndpointCard
      editable={canEdit}
      gatewayManaged={api.readOnly === true}
      editOperation="UpdateGraphQLAPI"
      onSave={(endpointUrl) => {
        if (!api.id || sdl === undefined) {
          return Promise.reject(new Error('The API or its schema has not loaded yet.'));
        }
        return updateApi.mutateAsync({
          graphqlApiId: api.id,
          body: {
            metadata: {
              ...api,
              ...resuppliedSchemaSource(api, sdl),
              upstream: { ...api.upstream, main: { ...api.upstream?.main, url: endpointUrl } },
            },
          },
        });
      }}
      saving={updateApi.isPending}
      title={<FormattedMessage {...messages.endpointTitle} />}
      url={url}
    />
  );
}

export function GraphqlOverviewTab({
  api,
  sdl,
  deployedGateways,
  deployments,
}: {
  api: GraphQLApiDetail;
  sdl?: string;
  deployedGateways: Gateway[];
  deployments: Deployment[];
}) {
  const { orgHandle = '', projectHandler = '' } = useParams();
  const deployed = deployedGateways.length > 0;

  return (
    <Grid container spacing={2}>
      <Grid size={{ lg: 8, xs: 12 }}>
        <Stack spacing={2} marginTop={1}>
          <Card>
            {/* An API imported from a gateway carries no SDL, and the server
                returns it as an empty string — that's "no schema yet", not an
                unparseable one, so the explorer gets its initial empty state. */}
            <GraphqlSchemaExplorer sdl={sdl?.trim() ? sdl : undefined} variant="card" />
          </Card>
          <DocumentsPanel
            apiHandle={api.id}
            apiType={GRAPHQL_API_TYPE}
            documentsPath={
              api.id
                ? routes.graphqlApiDevelopDocuments(orgHandle, projectHandler, api.id)
                : undefined
            }
          />
        </Stack>
      </Grid>
      <Grid size={{ lg: 4, xs: 12 }}>
        <Stack spacing={2} marginTop={1}>
          {deployed && (
            <>
              <Card sx={{ p: 2 }}>
                <Stack spacing={2}>
                  <InvokeUrlPanel
                    context={api.context}
                    gateways={deployedGateways}
                    version={api.version}
                  />
                  <Box sx={{ borderTop: '1px solid', borderColor: 'divider', pt: 2 }}>
                    <ApiKeysPanel apiId={api.id ?? ''} apiKind="graphql" />
                  </Box>
                </Stack>
              </Card>
              <DeployedGatewaysPanel
                apiId={api.id ?? ''}
                deployTo={routes.graphqlApiDeploy}
                deployments={deployments}
                gateways={deployedGateways}
              />
            </>
          )}
          <EndpointPanel api={api} sdl={sdl} />
        </Stack>
      </Grid>
    </Grid>
  );
}
