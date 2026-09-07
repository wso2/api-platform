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

import { alpha, Avatar, Box, Chip, Stack, Typography, useTheme } from '@wso2/oxygen-ui';
import { Clock, Network } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { Gateway } from '@/api/resources/gateways';
import { relativeTime } from '@/utils/relativeTime';
import { gatewayMode, type GatewayFunctionality } from '../utils/gatewayDisplay';

/**
 * Shared chips for gateway cards and rows. Connection state is the only accent;
 * everything else stays neutral for easy scanning.
 */

const messages = defineMessages({
  connected: {
    id: 'gatewayCard.connected',
    defaultMessage: 'Connected',
    description: 'Status shown when the gateway is talking to the platform.',
  },
  disconnected: {
    id: 'gatewayCard.disconnected',
    defaultMessage: 'Not connected',
    description: 'Status shown when the gateway is not reachable.',
  },
  functionalityAi: {
    id: 'gatewayCard.functionality.ai',
    defaultMessage: 'AI',
    description: 'Chip for a gateway provisioned to serve AI traffic.',
  },
  functionalityEvent: {
    id: 'gatewayCard.functionality.event',
    defaultMessage: 'Event',
    description: 'Chip for a gateway provisioned to serve event traffic.',
  },
  functionalityRegular: {
    id: 'gatewayCard.functionality.regular',
    defaultMessage: 'Regular',
    description: 'Chip for a gateway serving ordinary HTTP API traffic.',
  },
  modeManaged: {
    id: 'gatewayCard.mode.managed',
    defaultMessage: 'WSO2-managed',
    description: 'Chip for a gateway WSO2 runs on the customer’s behalf.',
  },
  modeSelfHosted: {
    id: 'gatewayCard.mode.selfHosted',
    defaultMessage: 'Self-hosted',
    description: 'Chip for a gateway the customer runs on their own infra.',
  },
  updated: {
    id: 'gatewayCard.updated',
    defaultMessage: 'Updated {relative}',
    description: 'Card footer timestamp; {relative} is a phrase such as "3 hours ago".',
  },
});

/** One label per `functionalityType` the spec allows; no raw enum on screen. */
const FUNCTIONALITY_LABEL: Record<GatewayFunctionality, typeof messages.functionalityRegular> = {
  ai: messages.functionalityAi,
  event: messages.functionalityEvent,
  regular: messages.functionalityRegular,
};

/** One neutral weight for descriptive chips; no icons or colours to avoid implying rank. */
const softChipSx = { flexShrink: 0, fontWeight: 500, typography: 'caption' } as const;

/** Icon-to-tile ratio, shared with the API card for consistent proportions. */
const AVATAR_ICON_RATIO = 0.57;

/** The gateway, as a square tile. Same tone in card and row. */
export function GatewayAvatar({ size = 56 }: { size?: number }) {
  return (
    <Avatar
      sx={{
        bgcolor: 'primary.light',
        color: 'primary.contrastText',
        flexShrink: 0,
        height: size,
        width: size,
      }}
      variant="rounded"
    >
      <Network size={Math.round(size * AVATAR_ICON_RATIO)} />
    </Avatar>
  );
}

/**
 * The gateway's version, under its name.
 */
export function GatewayVersion({ version }: { version?: string }) {
  if (!version) return null;

  return (
    <Typography color="text.secondary" component="div" noWrap variant="caption">
      {`v${version}`}
    </Typography>
  );
}

/** Whether the customer runs this gateway, or WSO2 does. */
export function GatewayModeChip({ gateway }: { gateway: Gateway }) {
  const { formatMessage } = useIntl();
  const isSelfHosted = gatewayMode(gateway) === 'self-hosted';

  return (
    <Chip
      label={formatMessage(isSelfHosted ? messages.modeSelfHosted : messages.modeManaged)}
      size="small"
      sx={softChipSx}
      variant="filled"
    />
  );
}

/** The kind of traffic the gateway was provisioned to serve. */
export function GatewayFunctionalityChip({ gateway }: { gateway: Gateway }) {
  const { formatMessage } = useIntl();

  if (!gateway.functionalityType) return null;

  return (
    <Chip
      label={formatMessage(FUNCTIONALITY_LABEL[gateway.functionalityType])}
      size="small"
      sx={softChipSx}
      variant="filled"
    />
  );
}

/** Diameter of the status dot. */
const STATUS_DOT_SIZE = 8;

/**
 * Whether the gateway is currently talking to the platform.
 */
export function GatewayStatusLabel({ gateway }: { gateway: Gateway }) {
  const { formatMessage } = useIntl();
  const theme = useTheme();
  const tone = gateway.isActive ? theme.palette.success.main : theme.palette.warning.main;

  return (
    <Chip
      label={
        <Stack alignItems="center" component="span" direction="row" spacing={0.75}>
          <Box
            component="span"
            sx={{
              bgcolor: 'currentColor',
              borderRadius: '50%',
              flexShrink: 0,
              height: STATUS_DOT_SIZE,
              width: STATUS_DOT_SIZE,
            }}
          />
          <Typography variant="caption">
            {formatMessage(gateway.isActive ? messages.connected : messages.disconnected)}
          </Typography>
        </Stack>
      }
      size="small"
      sx={{
        bgcolor: alpha(tone, 0.16),
        color: tone,
        flexShrink: 0,
        fontWeight: 600,
        typography: 'caption',
      }}
      variant="filled"
    />
  );
}

/**
 * When the gateway last changed. Renders the slot even with no timestamp, so a
 * card footer's space-between layout does not collapse.
 */
export function GatewayUpdatedLabel({ timestamp }: { timestamp?: string }) {
  return (
    <Stack
      alignItems="center"
      direction="row"
      spacing={0.75}
      sx={{ color: 'text.secondary', flexShrink: 0 }}
    >
      {timestamp && (
        <>
          <Clock size={14} />
          <Typography color="text.secondary" noWrap variant="caption">
            <FormattedMessage
              {...messages.updated}
              values={{ relative: relativeTime(timestamp) }}
            />
          </Typography>
        </>
      )}
    </Stack>
  );
}
