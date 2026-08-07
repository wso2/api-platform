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

import {
  useGatewayDeployments,
  useGateways,
} from '../../../api/hooks/useMvpQueries';
import type { ApiDetail, Gateway } from '../../../types/domain';
import { ApiKeysPanel } from './ApiKeysPanel';
import { InvokeUrlPanel } from './InvokeUrlPanel';
import { ProgressBanner } from './ProgressBanner';
import { ResourcesPanel } from './ResourcesPanel';

/**
 * Overview tab, ai-workspace layout: resources on the left; invoke URL and
 * API keys on the right — the right column only appears once the API is
 * deployed on at least one gateway.
 */
export function OverviewTab({ detail }: { detail: ApiDetail }) {
  const gatewaysQuery = useGateways();
  const deploymentsQuery = useGatewayDeployments(detail);

  // Gateways with an active deployment of this API, most recent first.
  const deployedGateways = useMemo((): Gateway[] => {
    const gateways = gatewaysQuery.data || [];
    const deployments = deploymentsQuery.data || [];
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
      .filter((gateway) => latestByGateway.has(gateway.id))
      .sort(
        (a, b) =>
          (latestByGateway.get(b.id) || 0) - (latestByGateway.get(a.id) || 0)
      );
  }, [gatewaysQuery.data, deploymentsQuery.data]);

  return (
    <>
      <ProgressBanner deployed={deployedGateways.length > 0} detail={detail} />
      <Stack direction={{ md: 'row', xs: 'column' }} spacing={2}>
        <ResourcesPanel detail={detail} />
        {deployedGateways.length > 0 && (
          <>
            <Divider
              flexItem
              orientation="vertical"
              sx={{ display: { md: 'block', xs: 'none' } }}
            />
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Stack spacing={2}>
                <InvokeUrlPanel
                  context={detail.context}
                  gateways={deployedGateways}
                />
                {detail.kind === 'API_PROXY' && (
                  <>
                    <Divider />
                    <ApiKeysPanel api={detail} />
                  </>
                )}
              </Stack>
            </Box>
          </>
        )}
      </Stack>
    </>
  );
}
