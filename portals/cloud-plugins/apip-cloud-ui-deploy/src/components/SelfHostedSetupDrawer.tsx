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

import { useState, type FC } from 'react';
import { Box, Card, CardContent, Chip, Drawer, IconButton, Stack, Typography } from '@wso2/oxygen-ui';
import { Check, X } from '@wso2/oxygen-ui-icons-react';
import {
  DISTRIBUTION_NAME,
  GatewaySetupStepsPreV1_2,
  GatewaySetupStepsV1_2Plus,
  isGatewayV12OrAbove,
  RELEASE_PATH,
} from './GatewaySetupSteps';
import type { GatewayVersionEntry } from '../hostPort';

export type SelfHostedSetupDrawerProps = {
  open: boolean;
  gatewayName: string;
  environment: string;
  /** The release the gateway was created at, from the host's config. */
  versionEntry?: GatewayVersionEntry;
  /** `host:port` the gateway's controller dials to reach the control plane. */
  controlPlaneHost: string;
  /** The registration token minted at creation; shown once, regenerated on demand. */
  initialToken: string | null;
  /** Whether the gateway has connected yet (its controller dialed in). */
  isActive: boolean;
  /** Mints a new token, revoking the previous one. */
  onGenerateToken: () => Promise<string>;
  onError: (message: string) => void;
  onClose: () => void;
};

const SUCCESS = '#5cc98f';

/** Spinner + waiting text until the gateway connects, then a green check. */
const ConnectionStatus: FC<{ isActive: boolean }> = ({ isActive }) =>
  isActive ? (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: SUCCESS }}>
      <Check size={18} />
      <Typography variant="body2" sx={{ color: SUCCESS, fontWeight: 600 }}>
        Gateway connected
      </Typography>
    </Box>
  ) : (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
      <Box
        sx={{
          width: 18,
          height: 18,
          borderRadius: '50%',
          border: '2px solid',
          borderColor: 'divider',
          borderTopColor: 'warning.main',
          animation: 'apip-shg-spin 0.9s linear infinite',
        }}
      />
      <style>{'@keyframes apip-shg-spin { to { transform: rotate(360deg); } }'}</style>
      <Typography variant="body2" color="text.secondary">
        Waiting for the gateway to connect…
      </Typography>
    </Box>
  );

const PREREQUISITES = ['cURL installed', 'unzip installed', 'Docker installed and running'];

/** The right-hand drawer with the steps for connecting a newly added self-hosted gateway. */
const SelfHostedSetupDrawer: FC<SelfHostedSetupDrawerProps> = ({
  open,
  gatewayName,
  environment,
  versionEntry,
  controlPlaneHost,
  initialToken,
  isActive,
  onGenerateToken,
  onError,
  onClose,
}) => {
  const [token, setToken] = useState<string | null>(initialToken);
  const [regenerated, setRegenerated] = useState(false);
  const [regenerating, setRegenerating] = useState(false);

  const gatewayVersion = versionEntry?.latestVersion ?? `v${versionEntry?.version ?? '1.0.0'}`;
  const distVersion = versionEntry?.distVersion ?? gatewayVersion.replace(/^v/, '');
  // The release path and the zip's own name differ, and both follow the gateway's type.
  const gatewayKind = versionEntry?.type ?? 'regular';
  const zipName = `wso2apip-${DISTRIBUTION_NAME[gatewayKind]}-${distVersion}`;

  const handleRegenerate = async () => {
    setRegenerating(true);
    try {
      setToken(await onGenerateToken());
      setRegenerated(true);
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Unable to generate a registration token.');
    } finally {
      setRegenerating(false);
    }
  };

  const stepsProps = {
    gatewayVersion,
    gatewayZipName: zipName,
    gatewayFolderName: zipName,
    releaseChannel: RELEASE_PATH[gatewayKind],
    controlPlaneHost,
    registrationToken: token,
    hasJustRegeneratedToken: regenerated,
    isRegeneratingToken: regenerating,
    onRegenerateToken: () => void handleRegenerate(),
  };

  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      slotProps={{ paper: { sx: { width: { xs: '100%', sm: 640 }, p: 3 } } }}
    >
      <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1, mb: 2 }}>
        <Typography variant="h6" sx={{ fontWeight: 700, flex: 1 }}>
          Set up {gatewayName}
        </Typography>
        <IconButton size="small" onClick={onClose} aria-label="Close">
          <X size={18} />
        </IconButton>
      </Box>

      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Stack direction="row" spacing={1}>
            <Chip size="small" variant="outlined" label={environment} />
            {versionEntry ? (
              <Chip size="small" variant="outlined" label={`Gateway v${versionEntry.version}`} />
            ) : null}
          </Stack>
        </CardContent>
      </Card>

      <Stack spacing={3}>
        <Box>
          <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>
            Prerequisites
          </Typography>
          <Box component="ul" sx={{ m: 0, pl: 3 }}>
            {PREREQUISITES.map((item) => (
              <Typography key={item} component="li" variant="body2" color="text.secondary">
                {item}
              </Typography>
            ))}
          </Box>
        </Box>
        {isGatewayV12OrAbove(distVersion) ? (
          <GatewaySetupStepsV1_2Plus {...stepsProps} />
        ) : (
          <GatewaySetupStepsPreV1_2 {...stepsProps} />
        )}

        <Card sx={{ p: 2.5 }}>
          <ConnectionStatus isActive={isActive} />
        </Card>
      </Stack>
    </Drawer>
  );
};

export default SelfHostedSetupDrawer;
