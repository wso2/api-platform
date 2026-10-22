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

/**
 * Self-hosted gateway setup/registration steps, split by gateway version —
 * ported from `ai-workspace/src/pages/appShell/appShellPages/gateways/GatewaySetupSteps.tsx`,
 * the AI Workspace's own native (non-plugin) gateway overview page, so this
 * plugin's Quick Start panel reads the same way. The version split, command
 * text and copy affordances are unchanged; only the control-plane host
 * (same-origin here, rather than an AI-Workspace-specific config constant)
 * and field styling (plain monospace, no `colorScheme` theming helper) adapt
 * it to this package.
 *
 * Gateway v1.2+ provisions certs/keys/env via `scripts/setup.sh`, delivers config through
 * `api-platform.env` (loaded by the compose `env_file:` block), and uses the canonical
 * `APIP_GW_CONTROLLER_*` config-interpolation env names.
 *
 * Older gateways (< v1.2) create `configs/keys.env` directly, use the legacy `GATEWAY_*`
 * env names, and start with `docker compose --env-file configs/keys.env up`.
 */

import type { ReactNode } from 'react';
import { Alert, Box, Button, IconButton, TextField, Typography } from '@wso2/oxygen-ui';
import { Copy } from '@wso2/oxygen-ui-icons-react';

/**
 * True for gateway v1.2 and above (the setup.sh + api-platform.env flow). `gatewayVersionHelm`
 * is the version without a leading "v" (e.g. "1.2.0"). Unparseable input defaults to the
 * current (v1.2+) flow.
 */
export const isGatewayV12OrAbove = (gatewayVersionHelm: string): boolean => {
  const [maj, min] = gatewayVersionHelm.split('.');
  const major = parseInt(maj ?? '', 10);
  const minor = parseInt(min ?? '', 10);
  if (Number.isNaN(major) || Number.isNaN(minor)) return true;
  return major > 1 || (major === 1 && minor >= 2);
};

export type GatewaySetupStepsProps = {
  /** Resolved version string (with leading "v"), used in the download URL. */
  gatewayVersion: string;
  gatewayZipName: string;
  gatewayFolderName: string;
  /** `releases/download/<releaseChannel>/...` — `ai-gateway`, `api-gateway` or `event-gateway`. */
  releaseChannel: string;
  registrationToken: string | null;
  hasJustRegeneratedToken: boolean;
  isRegeneratingToken: boolean;
  onRegenerateToken: () => void;
};

/** A read-only command box with a copy button (and optional distinct copy value). */
function CommandField({
  value,
  copyValue,
  copyLabel,
  minRows,
}: {
  value: string;
  /** Text placed on the clipboard when copied; defaults to `value`. */
  copyValue?: string;
  copyLabel: string;
  minRows?: number;
}) {
  return (
    <TextField
      fullWidth
      multiline={typeof minRows === 'number'}
      minRows={minRows}
      value={value}
      slotProps={{
        input: {
          readOnly: true,
          sx: { fontFamily: 'monospace', fontSize: '0.8125rem' },
          endAdornment: (
            <IconButton
              size="small"
              aria-label={`Copy ${copyLabel}`}
              onClick={() => {
                void navigator.clipboard.writeText(copyValue ?? value);
              }}
            >
              <Copy size={16} />
            </IconButton>
          ),
        },
      }}
    />
  );
}

/** Download the distribution — identical across versions. */
function DownloadStep({
  gatewayVersion,
  gatewayZipName,
  releaseChannel,
}: {
  gatewayVersion: string;
  gatewayZipName: string;
  releaseChannel: string;
}) {
  const downloadCommand = `curl -sLO https://github.com/wso2/api-platform/releases/download/${releaseChannel}/${gatewayVersion}/${gatewayZipName}.zip && \\
unzip ${gatewayZipName}.zip`;
  return (
    <Box>
      <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
        Step 1: Download the Gateway
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Run this command in your terminal to download the gateway:
      </Typography>
      <CommandField value={downloadCommand} copyLabel="Download command" minRows={2} />
    </Box>
  );
}

/** "Reconfigure" prompt shown when there is no active registration token. */
function ReconfigurePrompt({
  isRegeneratingToken,
  onRegenerateToken,
}: {
  isRegeneratingToken: boolean;
  onRegenerateToken: () => void;
}) {
  return (
    <>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Your existing local gateway can still use the previously generated token. If you want to
        generate a new token and new configuration command, click Reconfigure. This will revoke
        the previous token.
      </Typography>
      <Button variant="outlined" color="primary" onClick={onRegenerateToken} disabled={isRegeneratingToken}>
        Reconfigure
      </Button>
    </>
  );
}

// Same-origin as the console itself — the host this gateway's own controller
// dials out to, matching how `apiFetch` already reaches platform-api same-origin
// (see `RegistrationTokenDialog`, which this page's connection flow replaces).
const controlPlaneHost = (): string => (typeof window !== 'undefined' ? window.location.origin : '');

/**
 * Setup steps for gateway v1.2 and above:
 * 1. Download → 2. Set up (setup.sh) → 3. Configure (api-platform.env) → 4. Start.
 */
export function GatewaySetupStepsV1_2Plus({
  gatewayVersion,
  gatewayZipName,
  gatewayFolderName,
  releaseChannel,
  registrationToken,
  hasJustRegeneratedToken,
  isRegeneratingToken,
  onRegenerateToken,
}: GatewaySetupStepsProps): ReactNode {
  const envFile = 'api-platform.env';
  const setupCommand = `cd ${gatewayFolderName} && ./scripts/setup.sh`;
  const buildConfigureCommand = (token: string) =>
    `cat >> ${envFile} << 'ENVFILE'
APIP_GW_CONTROLLER_CONTROLPLANE_HOST=${controlPlaneHost()}
APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN=${token}
ENVFILE`;
  const startCommand = 'docker compose up';

  return (
    <>
      <DownloadStep gatewayVersion={gatewayVersion} gatewayZipName={gatewayZipName} releaseChannel={releaseChannel} />

      <Box>
        <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
          Step 2: Set up the Gateway
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Navigate into the gateway folder and run the setup script. It provisions the listener
          TLS certificate, the AES-256 at-rest encryption key, and the {envFile} file:
        </Typography>
        <CommandField value={setupCommand} copyLabel="Setup command" />
      </Box>

      <Box>
        <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
          Step 3: Configure the Gateway
        </Typography>
        {registrationToken ? (
          <>
            {hasJustRegeneratedToken && (
              <Alert severity="success" sx={{ mb: 2 }}>
                Successfully generated new configurations. Use the updated command below to
                reconfigure your gateway.
              </Alert>
            )}
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Run this command to add the control plane connection settings to {envFile}:
            </Typography>
            <CommandField
              value={buildConfigureCommand('<your-gateway-token>')}
              copyValue={buildConfigureCommand(registrationToken)}
              copyLabel="Configure command"
              minRows={4}
            />
          </>
        ) : (
          <ReconfigurePrompt isRegeneratingToken={isRegeneratingToken} onRegenerateToken={onRegenerateToken} />
        )}
      </Box>

      <Box>
        <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
          Step 4: Start the Gateway
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ my: 2 }}>
          From the gateway folder, run this command to start the gateway using the {envFile} file:
        </Typography>
        <CommandField value={startCommand} copyLabel="Start command" />
      </Box>
    </>
  );
}

/**
 * Setup steps for gateways below v1.2 (legacy):
 * 1. Download → 2. Configure (configs/keys.env) → 3. Start (navigate + --env-file).
 */
export function GatewaySetupStepsPreV1_2({
  gatewayVersion,
  gatewayZipName,
  gatewayFolderName,
  releaseChannel,
  registrationToken,
  hasJustRegeneratedToken,
  isRegeneratingToken,
  onRegenerateToken,
}: GatewaySetupStepsProps): ReactNode {
  const envFile = `${gatewayFolderName}/configs/keys.env`;
  const buildConfigureCommand = (token: string) =>
    `cat > ${envFile} << 'ENVFILE'
GATEWAY_CONTROLPLANE_HOST=${controlPlaneHost()}
GATEWAY_REGISTRATION_TOKEN=${token}
ENVFILE`;
  const navigateCommand = `cd ${gatewayFolderName}`;
  const startCommand = 'docker compose --env-file configs/keys.env up';

  return (
    <>
      <DownloadStep gatewayVersion={gatewayVersion} gatewayZipName={gatewayZipName} releaseChannel={releaseChannel} />

      <Box>
        <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
          Step 2: Configure the Gateway
        </Typography>
        {registrationToken ? (
          <>
            {hasJustRegeneratedToken && (
              <Alert severity="success" sx={{ mb: 2 }}>
                Successfully generated new configurations. Use the updated command below to
                reconfigure your gateway.
              </Alert>
            )}
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Run this command to create {envFile} with the required environment variables:
            </Typography>
            <CommandField
              value={buildConfigureCommand('<your-gateway-token>')}
              copyValue={buildConfigureCommand(registrationToken)}
              copyLabel="Configure command"
              minRows={4}
            />
          </>
        ) : (
          <ReconfigurePrompt isRegeneratingToken={isRegeneratingToken} onRegenerateToken={onRegenerateToken} />
        )}
      </Box>

      <Box>
        <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
          Step 3: Start the Gateway
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
          1. Navigate to the gateway folder.
        </Typography>
        <CommandField value={navigateCommand} copyLabel="Navigate command" />
        <Typography variant="body2" color="text.secondary" sx={{ my: 2 }}>
          2. Run this command to start the gateway using the configs/keys.env file created in
          Step 2:
        </Typography>
        <CommandField value={startCommand} copyLabel="Start command" />
      </Box>
    </>
  );
}
