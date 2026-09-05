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

import { useMemo } from 'react';
import { Box, Divider, Stack } from '@wso2/oxygen-ui';

import { useGateways, type Gateway } from '@/api/resources/gateways';
import type { RestApi } from '@/api/resources/restApis';
import { useDeployments } from '@/api/resources/restApis/deployments';
import { ApiKeysPanel } from './ApiKeysPanel';
import { InvokeUrlPanel } from './InvokeUrlPanel';
import { ProgressBanner } from './ProgressBanner';
import { ResourcesPanel } from './ResourcesPanel';

/**
 * The whole fleet in one request: the deployed set is filtered out of it, so a
 * default 20-item page could hide the very gateway this API runs on. 100 is the
 * spec's ceiling on `limit`.
 */
const GATEWAY_PAGE_LIMIT = 100;

/**
 * Overview tab: resources on the left; invoke URL and
 * API keys on the right; the right column only appears once the API is
 * deployed on at least one gateway.
 */
export function OverviewTab({ api }: { api: RestApi }) {
  const gatewaysQuery = useGateways({ limit: GATEWAY_PAGE_LIMIT });
  const deploymentsQuery = useDeployments(api.id);

  // Gateways with an active deployment of this API, most recent first.
  const deployedGateways = useMemo((): Gateway[] => {
    const gateways = gatewaysQuery.data?.list ?? [];
    const deployments = deploymentsQuery.data?.list ?? [];
    const latestByGateway = new Map<string, number>();
    deployments
      .filter((deployment) => deployment.status === 'DEPLOYED')
      .forEach((deployment) => {
        const time = new Date(deployment.createdAt || 0).getTime();
        const current = latestByGateway.get(deployment.gatewayId);
        if (current === undefined || time > current) {
          latestByGateway.set(deployment.gatewayId, time);
        }
      });
    return gateways
      .filter((gateway) => latestByGateway.has(gateway.id ?? ''))
      .sort(
        (a, b) => (latestByGateway.get(b.id ?? '') || 0) - (latestByGateway.get(a.id ?? '') || 0),
      );
  }, [gatewaysQuery.data, deploymentsQuery.data]);

  return (
    <Stack spacing={3}>
      <ProgressBanner api={api} deployed={deployedGateways.length > 0} />
      <Stack direction={{ md: 'row', xs: 'column' }} spacing={2}>
        <ResourcesPanel api={api} />
        {deployedGateways.length > 0 && (
          <>
            <Divider
              flexItem
              orientation="vertical"
              sx={{ display: { md: 'block', xs: 'none' } }}
            />
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Stack spacing={2}>
                <InvokeUrlPanel context={api.context} gateways={deployedGateways} />
                {/* Keys are an API-proxy affordance; `getApiCapabilities` draws
                    the same line off `kind`. */}
                {api.kind === 'RestApi' && (
                  <>
                    <Divider />
                    <ApiKeysPanel restApiId={api.id ?? ''} />
                  </>
                )}
              </Stack>
            </Box>
          </>
        )}
      </Stack>
    </Stack>
  );
}
