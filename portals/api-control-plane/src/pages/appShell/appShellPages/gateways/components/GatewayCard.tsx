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

import { Card, CardContent, CardHeader, Stack, Typography } from '@wso2/oxygen-ui';

import type { Gateway } from '@/api/resources/gateways';
import { interactiveCardSx } from '@/theme';
import {
  GatewayAvatar,
  GatewayFunctionalityChip,
  GatewayModeChip,
  GatewayStatusLabel,
  GatewayUpdatedLabel,
  GatewayVersion,
} from './gatewayChips';

type GatewayCardProps = {
  gateway: Gateway;
  onOpen: (gateway: Gateway) => void;
};

/** Edge of the square identity tile; the icon inside scales with it. */
const AVATAR_SIZE = 56;

/**
 * Gateway card for the listing grid, rendering the spec's `GatewayResponse`.
 *
 * Three bands, each answering one question, in the order a user asks them:
 * *which gateway is this* (tile, name, version), *what state is it in and what
 * kind is it* (the status pill leading two same-weight neutral chips), *how
 * current is this* (timestamp).
 */
export function GatewayCard({ gateway, onOpen }: GatewayCardProps) {
  const updated = gateway.updatedAt || gateway.createdAt;

  return (
    <Card
      onClick={() => onOpen(gateway)}
      sx={{
        ...interactiveCardSx,
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
      }}
    >
      <CardHeader
        avatar={<GatewayAvatar size={AVATAR_SIZE} />}
        slotProps={{
          // Prevent long names from widening the card by adding overflow handling
          content: { sx: { minWidth: 0, overflow: 'hidden' } },
          subheader: { component: 'div' },
          title: { component: 'div' },
        }}
        subheader={<GatewayVersion version={gateway.version} />}
        sx={{ alignItems: 'center', pb: 0 }}
        title={
          // Use div to enable text truncation with ellipsis
          <Typography component="div" noWrap sx={{ fontWeight: 600 }} variant="subtitle1">
            {gateway.displayName}
          </Typography>
        }
      />

      <CardContent sx={{ display: 'flex', flex: 1, flexDirection: 'column', gap: 2, pt: 2 }}>
        <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', gap: 1 }} useFlexGap>
          <GatewayStatusLabel gateway={gateway} />
          <GatewayModeChip gateway={gateway} />
          <GatewayFunctionalityChip gateway={gateway} />
        </Stack>
        <Stack sx={{ mt: 'auto' }}>
          <GatewayUpdatedLabel timestamp={updated} />
        </Stack>
      </CardContent>
    </Card>
  );
}
