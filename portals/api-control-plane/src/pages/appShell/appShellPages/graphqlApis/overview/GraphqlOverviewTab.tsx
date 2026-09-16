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
import { defineMessages, FormattedMessage } from 'react-intl';

import type { Gateway } from '@/api/resources/gateways';
import type { GraphQLApiDetail } from '@/api/resources/graphqlApis';
import type { Deployment } from '@/api/resources/graphqlApis/deployments';
import { GraphqlSchemaExplorer } from '../../apis/create/components/graphql/GraphqlSchemaExplorer';
import { EndpointsPanel } from '../../apis/overview/EndpointsPanel';
import { InvokeUrlPanel } from '../../apis/overview/InvokeUrlPanel';
import { GraphqlApiKeysPanel } from './GraphqlApiKeysPanel';
import { GraphqlDeployedGatewaysPanel } from './GraphqlDeployedGatewaysPanel';

const messages = defineMessages({
  schemaTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.graphqlApis.overview.GraphqlOverviewTab.schemaTitle',
    defaultMessage: 'Schema',
  },
});

/**
 * Overview tab for a GraphQL API: schema explorer on the left, connectivity
 * details on the right — mirrors `apis/overview/OverviewTab.tsx`'s layout,
 * swapping the REST-only resources panel for the schema explorer already
 * built for the creation wizard, and API keys/deployed gateways for their
 * GraphQL-scoped forks. `EndpointsPanel` and `InvokeUrlPanel` are reused
 * unmodified — both are already generic over `{url}` / `{gateways, context}`.
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
          <Card sx={{ display: 'flex', flexDirection: 'column', minHeight: 480, p: 2 }}>
            <Typography sx={{ fontWeight: 600, mb: 1.5 }} variant="h6">
              <FormattedMessage {...messages.schemaTitle} />
            </Typography>
            <Box sx={{ flex: 1, minHeight: 0 }}>
              <GraphqlSchemaExplorer sdl={sdl} />
            </Box>
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
              <GraphqlDeployedGatewaysPanel deployments={deployments} gateways={deployedGateways} />
            </>
          )}
          <EndpointsPanel url={api.upstream?.main?.url} />
        </Stack>
      </Grid>
    </Grid>
  );
}
