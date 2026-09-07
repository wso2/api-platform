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

import { Box, Card, Stack, Typography } from '@wso2/oxygen-ui';

import type { Gateway } from '@/api/resources/gateways';
import {
  GatewayAvatar,
  GatewayFunctionalityChip,
  GatewayModeChip,
  GatewayStatusLabel,
  GatewayUpdatedLabel,
  GatewayVersion,
} from './gatewayChips';

/** Smaller than the card's tile, since a row has less height to fill. */
const AVATAR_SIZE = 40;

type GatewayRowProps = {
  gateway: Gateway;
  onOpen: (gateway: Gateway) => void;
};

/**
 * Renders a single gateway row with the same visual indicators as `GatewayCard`.
 */
function GatewayRow({ gateway, onOpen }: GatewayRowProps) {
  const updated = gateway.updatedAt || gateway.createdAt;

  return (
    <Box
      onClick={() => onOpen(gateway)}
      sx={(theme) => ({
        alignItems: 'center',
        borderBottom: `${theme.border.width} ${theme.border.style}`,
        borderColor: 'divider',
        cursor: 'pointer',
        display: 'flex',
        gap: 2,
        px: 2.5,
        py: 1.75,
        transition: theme.transitions.create('background-color'),
        '&:hover': { bgcolor: 'action.hover' },
        '&:last-of-type': { borderBottom: 0 },
      })}
    >
      <GatewayAvatar size={AVATAR_SIZE} />

      {/* Allows long names to truncate. Version below name for layout. */}
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography component="div" noWrap sx={{ fontWeight: 600 }} variant="subtitle2">
          {gateway.displayName}
        </Typography>
        <GatewayVersion version={gateway.version} />
      </Box>

      {/* Dropped first on narrow rows. */}
      <Stack
        direction="row"
        spacing={1}
        sx={{
          display: { sm: 'flex', xs: 'none' },
          flexShrink: 0,
          flexWrap: 'wrap',
          gap: 1,
          justifyContent: 'flex-end',
        }}
        useFlexGap
      >
        <GatewayModeChip gateway={gateway} />
        <GatewayFunctionalityChip gateway={gateway} />
      </Stack>

      <GatewayStatusLabel gateway={gateway} />

      <Box sx={{ display: { md: 'flex', xs: 'none' } }}>
        <GatewayUpdatedLabel timestamp={updated} />
      </Box>
    </Box>
  );
}

type GatewayListViewProps = {
  gateways: Gateway[];
  onOpen: (gateway: Gateway) => void;
};

/** Compact row layout for one environment group, counterpart of the card grid. */
export function GatewayListView({ gateways, onOpen }: GatewayListViewProps) {
  return (
    <Card data-testid="gateway-list-view" variant="outlined">
      {gateways.map((gateway) => (
        <GatewayRow gateway={gateway} key={gateway.id} onOpen={onOpen} />
      ))}
    </Card>
  );
}
