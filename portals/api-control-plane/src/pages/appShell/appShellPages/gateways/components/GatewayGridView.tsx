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

import { Box } from '@wso2/oxygen-ui';

import type { Gateway } from '@/api/resources/gateways';
import { GatewayCard } from './GatewayCard';

type GatewayGridViewProps = {
  gateways: Gateway[];
  onOpen: (gateway: Gateway) => void;
};

/** Card grid for one environment group, same density as the API listing. */
export function GatewayGridView({ gateways, onOpen }: GatewayGridViewProps) {
  return (
    <Box
      data-testid="gateway-grid-view"
      sx={{
        display: 'grid',
        gap: 2.5,
        gridTemplateColumns: {
          xs: '1fr',
          sm: 'repeat(2, 1fr)',
          md: 'repeat(3, 1fr)',
          lg: 'repeat(4, 1fr)',
        },
        // Allow cards to shrink so long names do not widen the grid.
        '& > *': { minWidth: 0 },
      }}
    >
      {gateways.map((gateway) => (
        <GatewayCard gateway={gateway} key={gateway.id} onOpen={onOpen} />
      ))}
    </Box>
  );
}
