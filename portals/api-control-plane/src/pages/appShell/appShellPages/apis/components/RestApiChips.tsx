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

import { Box, Chip, Stack, Typography, useTheme } from '@wso2/oxygen-ui';
import type { Theme } from '@wso2/oxygen-ui';
import { Globe, Lock } from '@wso2/oxygen-ui-icons-react';

import {
  deploymentMeta,
  lifecycleMeta,
  type ApiDeploymentState,
  type ChipColor,
  type LifeCycleStatus,
} from '../restApiDisplay';

/**
 * The status marks shared by the API card (grid) and the API row (list), so the
 * two views can never drift on what "Published" or "Active" looks like.
 */

/**
 * Metadata chips run a step smaller than MUI's `size="small"`, which is scaled
 * for chips you click. These are read-only marks sitting under a card title, so
 * they take the theme's `caption` scale — one definition, applied by every chip
 * here, and no font-size literal.
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
      sx={{ flexShrink: 0, fontFamily: 'monospace', fontSize: "0.7rem" }}
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
            sx={{ ...metaChipSx, fontSize: "0.7rem" }}
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
  const main =
    color === 'default' ? theme.palette.text.disabled : tone(theme, color);

  return (
    <Stack
      alignItems="center"
      direction="row"
      spacing={0.875}
      sx={{ color: main, flexShrink: 0 }}
    >
      <Box
        sx={{ bgcolor: main, borderRadius: '50%', height: 8, width: 8 }}
      />
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
        <Chip
          key={gatewayId}
          label={gatewayId}
          size="small"
          sx={metaChipSx}
          variant="outlined"
        />
      ))}
    </>
  );
}
