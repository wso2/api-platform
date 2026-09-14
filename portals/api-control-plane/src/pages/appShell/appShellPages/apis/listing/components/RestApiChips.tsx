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

import {
  Avatar,
  Box,
  Chip,
  IconButton,
  Stack,
  Tooltip,
  Typography,
  useTheme,
} from '@wso2/oxygen-ui';
import type { Theme } from '@wso2/oxygen-ui';
import { Boxes, Clock, Globe, Lock, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { relativeTime } from '@/utils/relativeTime';
import {
  apiKindLabel,
  deploymentMeta,
  lifecycleMeta,
  type ApiDeploymentState,
  type ChipColor,
  type LifeCycleStatus,
} from '../../utils/restApiDisplay';

const messages = defineMessages({
  deleteAction: {
    id: 'apiCard.deleteAction',
    defaultMessage: 'Delete API',
    description: 'Delete action shown in an API card or row actions menu.',
  },
  deleteAriaLabel: {
    id: 'apiCard.deleteAriaLabel',
    defaultMessage: 'Delete {apiName}',
    description: 'Accessible label for the API delete button.',
  },
  gatewayManagedLabel: {
    id: 'apiCard.gatewayManaged.label',
    defaultMessage: 'Gateway-managed',
    description: 'Chip marking an API the console discovered from a gateway and cannot edit.',
  },
  gatewayManagedTooltip: {
    id: 'apiCard.gatewayManaged.tooltip',
    defaultMessage: 'Discovered from a data-plane gateway — read-only here',
    description: 'Explains why a gateway-managed API cannot be edited here.',
  },
  updated: {
    id: 'apiCard.updated',
    defaultMessage: 'Updated {relative}',
    description: 'Card footer timestamp; {relative} is a phrase such as "3 hours ago".',
  },
});

/**
 * The status marks shared by the API card (grid) and the API row (list), so the
 * two views can never drift on what "Published" or "Active" looks like.
 */

const metaChipSx = { typography: 'caption' } as const;

/** Resolves a semantic chip colour to the palette; `default` stays neutral. */
const tone = (theme: Theme, color: ChipColor): string =>
  color === 'default' ? theme.palette.text.secondary : theme.palette[color].main;

/** Lifecycle of the API definition itself — Published, Created, Deprecated… */
export function LifecycleChip({ status }: { status?: LifeCycleStatus }) {
  const { color, label } = lifecycleMeta(status);

  return (
    <Chip
      color={color}
      label={label}
      size="small"
      sx={{ ...metaChipSx, flexShrink: 0 }}
      variant="outlined"
    />
  );
}

/** Compact dot-and-label lifecycle treatment used by overview cards and rows. */
export function LifecycleStatusLabel({ status }: { status?: LifeCycleStatus }) {
  const theme = useTheme();
  const { color, label } = lifecycleMeta(status);
  const main = tone(theme, color);

  return (
    <Stack alignItems="center" direction="row" spacing={0.75} sx={{ color: main }}>
      <Box sx={{ bgcolor: main, borderRadius: '50%', height: 8, width: 8 }} />
      <Typography variant="caption">{label}</Typography>
    </Stack>
  );
}

/** API kind shown as a quiet chip beside the version. */
export function ApiKindChip({ kind }: { kind?: string }) {
  return <Chip label={apiKindLabel(kind)} size="small" sx={metaChipSx} />;
}

/**
 * The API's version, beside its name. Monospace so digits line up down a column
 * of cards; the chip's size, radius and border are the theme's.
 */
export function VersionChip({ version }: { version?: string }) {
  if (!version) return null;

  return (
    <Chip
      label={`v${version}`}
      size="small"
      sx={{ flexShrink: 0, fontSize: '0.7rem' }}
      variant="outlined"
    />
  );
}

/**
 * Icons per transport. `https` earns the padlock because the distinction the
 * user cares about is whether the hop is encrypted; anything the gateway
 * reports that we have no icon for falls back to the neutral globe rather than
 * rendering an iconless odd-one-out.
 */
const TRANSPORT_ICON: Record<string, typeof Globe> = {
  http: Globe,
  https: Lock,
};

/** Protocols the API is exposed over — `HTTP`, `HTTPS`. */
export function TransportChips({ transports }: { transports: string[] }) {
  return (
    <>
      {transports.map((transport) => {
        const Icon = TRANSPORT_ICON[transport.toLowerCase()] ?? Globe;
        return (
          <Chip
            icon={<Icon size={12} />}
            key={transport}
            label={transport.toUpperCase()}
            size="small"
            sx={{ ...metaChipSx, fontSize: '0.7rem' }}
            variant="filled"
          />
        );
      })}
    </>
  );
}

/**
 * Runtime state across the API's gateways — a dot plus a word, deliberately
 * quieter than the lifecycle chip since it sits in the card footer.
 */
export function DeploymentStateLabel({ state }: { state: ApiDeploymentState }) {
  const theme = useTheme();
  const { color, label } = deploymentMeta(state);
  const main = color === 'default' ? theme.palette.text.disabled : tone(theme, color);

  return (
    <Stack alignItems="center" direction="row" spacing={0.875} sx={{ color: main, flexShrink: 0 }}>
      <Box sx={{ bgcolor: main, borderRadius: '50%', height: 8, width: 8 }} />
      <Typography sx={{ fontSize: 12.5, fontWeight: 500 }}>{label}</Typography>
    </Stack>
  );
}

/**
 * Gateways the API is live on, labelled by gateway handle. Renders nothing when
 * it is deployed nowhere — an empty rail reads as a loading glitch, and the
 * neighbouring state label already says "Not deployed".
 */
export function GatewayChips({ gatewayIds }: { gatewayIds: string[] }) {
  if (gatewayIds.length === 0) return null;

  return (
    <>
      {gatewayIds.map((gatewayId) => (
        <Chip key={gatewayId} label={gatewayId} size="small" sx={metaChipSx} variant="outlined" />
      ))}
    </>
  );
}

/* -------------------------------------------------------------------------- */
/* Shared card/row furniture                                                  */
/* -------------------------------------------------------------------------- */

/**
 * Ratio between the kind tile's edge and the icon inside it, so a row-sized
 * tile keeps the grid card's proportions instead of guessing a second pair of
 * numbers.
 */
const KIND_AVATAR_ICON_RATIO = 0.57;

/**
 * The API's kind, as a square tile. Same tone in both views — a row and a card
 * for the same API must not read as two different things.
 */
export function ApiKindAvatar({ kind, size = 56 }: { kind?: string; size?: number }) {
  return (
    <Tooltip placement="left" title={apiKindLabel(kind)}>
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
        <Boxes size={Math.round(size * KIND_AVATAR_ICON_RATIO)} />
      </Avatar>
    </Tooltip>
  );
}

/** Marks an API the console can show but not edit. */
export function GatewayManagedChip() {
  const { formatMessage } = useIntl();

  return (
    <Tooltip title={formatMessage(messages.gatewayManagedTooltip)}>
      <Chip
        icon={<Lock size={12} />}
        label={formatMessage(messages.gatewayManagedLabel)}
        size="small"
        sx={{ flexShrink: 0, height: 24, p: 0, width: 24 }}
        variant="outlined"
      />
    </Tooltip>
  );
}

const DESCRIPTION_FONT_SIZE = '0.7rem';

/**
 * Description typography for a card (two lines) or a row (one) — clamped rather
 * than truncated mid-word, and one definition of the size so the two views
 * cannot drift.
 */
export const apiDescriptionSx = (lines: number) =>
  ({
    display: '-webkit-box',
    fontSize: DESCRIPTION_FONT_SIZE,
    overflow: 'hidden',
    WebkitBoxOrient: 'vertical',
    WebkitLineClamp: lines,
  }) as const;

/**
 * When the API last changed. Renders the row/footer slot even with no
 * timestamp, so a card footer's space-between layout does not collapse.
 */
export function UpdatedLabel({
  timestamp,
  withPrefix = true,
}: {
  timestamp?: string;
  withPrefix?: boolean;
}) {
  return (
    <Stack
      alignItems="center"
      direction="row"
      spacing={1}
      sx={{ color: 'text.secondary', flexShrink: 0 }}
    >
      {timestamp && (
        <>
          {withPrefix && <Clock size={16} />}
          <Typography color="text.secondary" noWrap variant="caption">
            {withPrefix ? (
              <FormattedMessage
                {...messages.updated}
                values={{ relative: relativeTime(timestamp) }}
              />
            ) : (
              relativeTime(timestamp)
            )}
          </Typography>
        </>
      )}
    </Stack>
  );
}

/** Direct delete action shared by API cards and rows. */
export function ApiDeleteButton({ apiName, onDelete }: { apiName: string; onDelete: () => void }) {
  const { formatMessage } = useIntl();

  return (
    <Tooltip title={formatMessage(messages.deleteAction)}>
      <IconButton
        aria-label={formatMessage(messages.deleteAriaLabel, { apiName })}
        className="api-delete-action"
        color="error"
        onClick={(event) => {
          event.stopPropagation();
          onDelete();
        }}
        size="small"
        sx={{
          flexShrink: 0,
          opacity: { md: 0, xs: 1 },
          transition: 'opacity 150ms ease',
        }}
      >
        <Trash2 size={18} />
      </IconButton>
    </Tooltip>
  );
}
