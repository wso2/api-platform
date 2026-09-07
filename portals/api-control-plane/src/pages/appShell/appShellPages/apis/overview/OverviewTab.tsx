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

import type { Gateway } from '@/api/resources/gateways';
import type { RestApi } from '@/api/resources/restApis';
import type { Deployment } from '@/api/resources/restApis/deployments';
import { ApiKeysPanel } from './ApiKeysPanel';
import { DeployedGatewaysPanel } from './DeployedGatewaysPanel';
import { DocumentsPanel } from './DocumentsPanel';
import { InvokeUrlPanel } from './InvokeUrlPanel';
import { ResourcesPanel } from './ResourcesPanel';

/**
 * The whole fleet in one request: the deployed set is filtered out of it, so a
 * default 20-item page could hide the very gateway this API runs on. 100 is the
 * spec's ceiling on `limit`.
 */
/**
 * Overview tab: resources on the left; invoke URL and
 * API keys on the right; the right column only appears once the API is
 * deployed on at least one gateway.
 */
export function OverviewTab({
  api,
  deployedGateways,
  deployments,
}: {
  api: RestApi;
  deployedGateways: Gateway[];
  deployments: Deployment[];
}) {
  const deployed = deployedGateways.length > 0;

  return (
    <Grid container spacing={2}>
      <Grid size={{ lg: deployed ? 8 : 12, xs: 12 }}>
        <Stack spacing={2} marginTop={1}>
          <ResourcesPanel api={api} />
          <DocumentsPanel />
        </Stack>
      </Grid>
      {deployed && (
        <Grid size={{ lg: 4, xs: 12 }}>
          <Stack spacing={2} marginTop={1}>
            <Card sx={{ p: 2 }}>
              <Stack spacing={2}>
                <InvokeUrlPanel context={api.context} gateways={deployedGateways} />
                {api.kind === 'RestApi' && (
                  <Box sx={{ borderTop: '1px solid', borderColor: 'divider', pt: 2 }}>
                    <ApiKeysPanel restApiId={api.id ?? ''} />
                  </Box>
                )}
              </Stack>
            </Card>
            <DeployedGatewaysPanel deployments={deployments} gateways={deployedGateways} />
          </Stack>
        </Grid>
      )}
    </Grid>
  );
}
