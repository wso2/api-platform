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
import { defineMessages, FormattedMessage } from 'react-intl';

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

const messages = defineMessages({
  gateway: {
    id: 'apiControlPlane.pages.appShell.appShellPages.gateways.components.GatewayListView.column.gateway',
    defaultMessage: 'Gateway',
    description: 'Column header over the gateway name.',
  },
  status: {
    id: 'apiControlPlane.pages.appShell.appShellPages.gateways.components.GatewayListView.column.status',
    defaultMessage: 'Status',
    description: 'Column header over the connected/not-connected indicator.',
  },
  type: {
    id: 'apiControlPlane.pages.appShell.appShellPages.gateways.components.GatewayListView.column.type',
    defaultMessage: 'Type',
    description: 'Column header over the hosting-mode and traffic-kind chips.',
  },
  updated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.gateways.components.GatewayListView.column.updated',
    defaultMessage: 'Updated',
    description: 'Column header over the last-updated timestamp.',
  },
  version: {
    id: 'apiControlPlane.pages.appShell.appShellPages.gateways.components.GatewayListView.column.version',
    defaultMessage: 'Version',
    description: 'Column header over the gateway version.',
  },
});

/**
 * Shared by the header and every row, so a label can never drift from the cells
 * under it. Below `md` only the name and status survive — the columns between
 * them are the first to go, as they are the ones a card would still show.
 */
const rowGridSx = {
  alignItems: 'center',
  display: 'grid',
  gap: 2,
  gridTemplateColumns: {
    xs: 'minmax(0, 1fr) auto',
    md: 'minmax(0, 1fr) 100px 220px 150px 180px',
  },
} as const;

/** Columns that collapse on a narrow row, hidden in both header and body. */
const wideOnlySx = { display: { md: 'block', xs: 'none' } } as const;
/** Same breakpoint, for a cell whose contents need to lay out as a row. */
const wideOnlyFlexSx = { display: { md: 'flex', xs: 'none' } } as const;

const headerLabelSx = { fontWeight: 700 } as const;

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
        borderBottom: `${theme.border.width} ${theme.border.style}`,
        borderColor: 'divider',
        cursor: 'pointer',
        px: 2.5,
        py: 1.75,
        transition: theme.transitions.create('background-color'),
        ...rowGridSx,
        '&:hover': { bgcolor: 'action.hover' },
        '&:last-of-type': { borderBottom: 0 },
      })}
    >
      {/* `minWidth: 0` is what lets a long name truncate instead of widening the grid. */}
      <Stack alignItems="center" direction="row" spacing={1.5} sx={{ minWidth: 0 }}>
        <GatewayAvatar size={AVATAR_SIZE} />
        <Typography component="div" noWrap sx={{ fontWeight: 600 }} variant="subtitle2">
          {gateway.displayName}
        </Typography>
      </Stack>

      {/* Out of the name cell and into its own column, so versions line up. */}
      <Box sx={wideOnlySx}>
        <GatewayVersion version={gateway.version} />
      </Box>

      <Stack direction="row" spacing={1} sx={wideOnlyFlexSx}>
        <GatewayModeChip gateway={gateway} />
        <GatewayFunctionalityChip gateway={gateway} />
      </Stack>

      {/* The one column that survives a narrow row — a gateway that is down is
          the reason to look at this list at all. */}
      <Box sx={{ display: 'flex' }}>
        <GatewayStatusLabel gateway={gateway} />
      </Box>

      <Box sx={wideOnlyFlexSx}>
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
      <Box sx={{ ...rowGridSx, bgcolor: 'action.hover', px: 2.5, py: 1.25 }}>
        <Typography color="text.secondary" sx={headerLabelSx} variant="caption">
          <FormattedMessage {...messages.gateway} />
        </Typography>
        <Typography
          color="text.secondary"
          sx={{ ...wideOnlySx, ...headerLabelSx }}
          variant="caption"
        >
          <FormattedMessage {...messages.version} />
        </Typography>
        <Typography
          color="text.secondary"
          sx={{ ...wideOnlySx, ...headerLabelSx }}
          variant="caption"
        >
          <FormattedMessage {...messages.type} />
        </Typography>
        <Typography color="text.secondary" sx={headerLabelSx} variant="caption">
          <FormattedMessage {...messages.status} />
        </Typography>
        <Typography
          color="text.secondary"
          sx={{ ...wideOnlySx, ...headerLabelSx }}
          variant="caption"
        >
          <FormattedMessage {...messages.updated} />
        </Typography>
      </Box>
      {gateways.map((gateway) => (
        <GatewayRow gateway={gateway} key={gateway.id} onOpen={onOpen} />
      ))}
    </Card>
  );
}
