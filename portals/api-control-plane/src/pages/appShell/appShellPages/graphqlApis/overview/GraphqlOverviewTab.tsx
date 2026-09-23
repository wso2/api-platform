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

import { Box, Card, Grid, Stack, Typography } from '@wso2/oxygen-ui';
import { Globe } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage } from 'react-intl';

import type { Gateway } from '@/api/resources/gateways';
import type { GraphQLApiDetail } from '@/api/resources/graphqlApis';
import type { Deployment } from '@/api/resources/graphqlApis/deployments';
import { routes } from '@/routes/paths';
import { GraphqlSchemaExplorer } from '../../apis/create/components/graphql/GraphqlSchemaExplorer';
import { DeployedGatewaysPanel } from '../../apis/overview/DeployedGatewaysPanel';
import { InvokeUrlPanel } from '../../apis/overview/InvokeUrlPanel';
import { GraphqlApiKeysPanel } from './GraphqlApiKeysPanel';

const messages = defineMessages({
  schemaTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.overview.GraphqlOverviewTab.schemaTitle',
    defaultMessage: 'Schema',
  },
  endpointTitle: {
    id: 'apiControlPlane.pages.test.console.GatewaySection.endpoint',
    defaultMessage: 'Endpoint',
    description: 'Label above the URL that requests from this console are sent to. Shown in capitals by the layout, so translate it as ordinary words.',
  },
  endpointNotConfigured: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.EndpointsPanel.notConfigured',
    defaultMessage: 'No endpoint configured',
  },
});

/**
 * Read-only backend-endpoint display for a GraphQL API. `EndpointsPanel`
 * (REST's Overview equivalent) grew inline editing backed by
 * `useUpdateRestApi` and now takes a whole `RestApi`, so it's no longer the
 * generic `{url}` component this page was written against — reusing it here
 * would wire a REST-only mutation to a GraphQL API's id. Editing a GraphQL
 * API's upstream endpoint has no equivalent flow yet, so this stays
 * display-only until one exists.
 */
const EndpointPanel = ({ url }: { url?: string }) => (
  <Card sx={{ p: 2 }}>
    <Stack spacing={1.5}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
        <Globe size={18} />
        <Typography sx={{ fontWeight: 600 }} variant="subtitle2">
          <FormattedMessage {...messages.endpointTitle} />
        </Typography>
      </Stack>
      <Typography color={url ? 'text.primary' : 'text.secondary'} sx={{ wordBreak: 'break-all' }} variant="body2">
        {url ?? <FormattedMessage {...messages.endpointNotConfigured} />}
      </Typography>
    </Stack>
  </Card>
);

/**
 * Overview tab for a GraphQL API: schema explorer on the left, connectivity
 * details on the right — mirrors `apis/overview/OverviewTab.tsx`'s layout,
 * swapping the REST-only resources panel for the schema explorer already
 * built for the creation wizard, and API keys for `GraphqlApiKeysPanel` (its
 * create-key flow is genuinely different from REST's, see that file).
 * `InvokeUrlPanel` and `DeployedGatewaysPanel` are reused unmodified — both
 * are generic over their own props/a route builder, not REST-specific.
 */
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
  const deployed = deployedGateways.length > 0;

  return (
    <Grid container spacing={2}>
      <Grid size={{ lg: 8, xs: 12 }}>
        <Stack spacing={2} marginTop={1}>
          <Card sx={{ display: 'flex', flexDirection: 'column', p: 2 }}>
            <Typography sx={{ fontWeight: 600, mb: 1.5 }} variant="h6">
              <FormattedMessage {...messages.schemaTitle} />
            </Typography>
            <GraphqlSchemaExplorer sdl={sdl} />
          </Card>
        </Stack>
      </Grid>
      <Grid size={{ lg: 4, xs: 12 }}>
        <Stack spacing={2} marginTop={1}>
          {deployed && (
            <>
              <Card sx={{ p: 2 }}>
                <Stack spacing={2}>
                  <InvokeUrlPanel context={api.context} gateways={deployedGateways} version={api.version} />
                  <Box sx={{ borderTop: '1px solid', borderColor: 'divider', pt: 2 }}>
                    <GraphqlApiKeysPanel graphqlApiId={api.id ?? ''} />
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
          <EndpointPanel url={api.upstream?.main?.url} />
        </Stack>
      </Grid>
    </Grid>
  );
}
