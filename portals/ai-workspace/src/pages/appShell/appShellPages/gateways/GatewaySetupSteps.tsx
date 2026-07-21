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
 * Self-hosted gateway setup/registration steps, split by gateway version.
 *
 * Gateway v1.2+ provisions certs/keys/env via `scripts/setup.sh`, delivers config through
 * `api-platform.env` (loaded by the compose `env_file:` block), and uses the canonical
 * `APIP_GW_CONTROLLER_*` config-interpolation env names.
 *
 * Older gateways (< v1.2) create `configs/keys.env` directly, use the legacy `GATEWAY_*`
 * env names, and start with `docker compose --env-file configs/keys.env up`.
 *
 * `ViewGateway` picks the matching component with {@link isGatewayV12OrAbove} and renders it
 * inside each compose install tab, so it holds no version-specific command logic itself.
 */

import { type ClipboardEvent, type ReactNode } from "react";
import {
  Box,
  Button,
  Typography,
  TextField,
  IconButton,
  Alert,
} from "@wso2/oxygen-ui";
import { Copy } from "@wso2/oxygen-ui-icons-react";
import { CONTROLPLANE_HOST } from "../../../../config.env";
import {
  getCommandTextFieldSx,
  type ColorScheme,
} from "../../../../utils/colorScheme";

/**
 * True for gateway v1.2 and above (the setup.sh + api-platform.env flow). `gatewayVersionHelm`
 * is the version without a leading "v" (e.g. "1.2.0-alpha2"). Unparseable input defaults to
 * the current (v1.2+) flow.
 */
export const isGatewayV12OrAbove = (gatewayVersionHelm: string): boolean => {
  const [maj, min] = gatewayVersionHelm.split(".");
  const major = parseInt(maj ?? "", 10);
  const minor = parseInt(min ?? "", 10);
  if (Number.isNaN(major) || Number.isNaN(minor)) return true;
  return major > 1 || (major === 1 && minor >= 2);
};

/** Bare env-file name for UI labels and downloads, per gateway version. */
export const getGatewayEnvFileName = (isV12OrAbove: boolean): string =>
  isV12OrAbove ? "api-platform.env" : "keys.env";

/** Raw env-file content (NAME=value lines) for the download button, per gateway version. */
export const buildGatewayEnvFileContent = (
  isV12OrAbove: boolean,
  controlPlaneHost: string,
  token: string,
): string =>
  isV12OrAbove
    ? `APIP_GW_CONTROLLER_CONTROLPLANE_HOST=${controlPlaneHost}\nAPIP_GW_CONTROLLER_CONTROLPLANE_TOKEN=${token}`
    : `GATEWAY_CONTROLPLANE_HOST=${controlPlaneHost}\nGATEWAY_REGISTRATION_TOKEN=${token}`;

export interface GatewaySetupStepsProps {
  /** Resolved version string (with leading "v"), used in the download URL. */
  gatewayVersion: string;
  gatewayZipName: string;
  gatewayFolderName: string;
  registrationToken: string | null;
  hasJustRegeneratedToken: boolean;
  isRegeneratingToken: boolean;
  onRegenerateToken: () => void;
  onCopy: (text: string, label: string) => void;
  colorScheme: ColorScheme;
  renderConnectionStatus: () => ReactNode;
}

/** A read-only command box with a copy button (and optional distinct copy value). */
function CommandField({
  value,
  copyValue,
  copyLabel,
  onCopy,
  colorScheme,
  minRows,
}: {
  value: string;
  /** Text placed on the clipboard when copied; defaults to `value`. */
  copyValue?: string;
  copyLabel: string;
  onCopy: (text: string, label: string) => void;
  colorScheme: ColorScheme;
  minRows?: number;
}) {
  const clipboardText = copyValue ?? value;
  const overrideManualCopy = copyValue !== undefined && copyValue !== value;
  return (
    <TextField
      fullWidth
      multiline={typeof minRows === "number"}
      minRows={minRows}
      value={value}
      sx={getCommandTextFieldSx(colorScheme)}
      onCopy={
        overrideManualCopy
          ? (e: ClipboardEvent<HTMLDivElement>) => {
              e.preventDefault();
              e.clipboardData.setData("text/plain", clipboardText);
            }
          : undefined
      }
      slotProps={{
        input: {
          readOnly: true,
          endAdornment: (
            <IconButton
              size="small"
              aria-label={`Copy ${copyLabel}`}
              onClick={() => onCopy(clipboardText, copyLabel)}
            >
              <Copy />
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
  onCopy,
  colorScheme,
}: {
  gatewayVersion: string;
  gatewayZipName: string;
  onCopy: (text: string, label: string) => void;
  colorScheme: ColorScheme;
}) {
  const downloadCommand = `curl -sLO https://github.com/wso2/api-platform/releases/download/ai-gateway/${gatewayVersion}/${gatewayZipName}.zip && \\
unzip ${gatewayZipName}.zip`;
  return (
    <Box>
      <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
        Step 1: Download the Gateway
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Run this command in your terminal to download the gateway:
      </Typography>
      <CommandField
        value={downloadCommand}
        copyLabel="Download command"
        onCopy={onCopy}
        colorScheme={colorScheme}
        minRows={2}
      />
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
        Your existing local gateway can still use the previously generated
        token. If you want to generate a new token and new configuration
        command, click Reconfigure. This will revoke the previous token.
      </Typography>
      <Button
        variant="outlined"
        color="primary"
        onClick={onRegenerateToken}
        disabled={isRegeneratingToken}
      >
        Reconfigure
      </Button>
    </>
  );
}

/**
 * Setup steps for gateway v1.2 and above:
 * 1. Download → 2. Set up (setup.sh) → 3. Configure (api-platform.env) → 4. Start.
 */
export function GatewaySetupStepsV1_2Plus({
  gatewayVersion,
  gatewayZipName,
  gatewayFolderName,
  registrationToken,
  hasJustRegeneratedToken,
  isRegeneratingToken,
  onRegenerateToken,
  onCopy,
  colorScheme,
  renderConnectionStatus,
}: GatewaySetupStepsProps) {
  const envFile = "api-platform.env";
  const setupCommand = `cd ${gatewayFolderName} && ./scripts/setup.sh`;
  const buildConfigureCommand = (token: string) =>
    `cat >> ${envFile} << 'ENVFILE'
APIP_GW_CONTROLLER_CONTROLPLANE_HOST=${CONTROLPLANE_HOST}
APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN=${token}
ENVFILE`;
  const startCommand = "docker compose up";

  return (
    <>
      <DownloadStep
        gatewayVersion={gatewayVersion}
        gatewayZipName={gatewayZipName}
        onCopy={onCopy}
        colorScheme={colorScheme}
      />

      {/* Step 2: Set up the Gateway */}
      <Box>
        <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
          Step 2: Set up the Gateway
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Navigate into the gateway folder and run the setup script. It
          provisions the listener TLS certificate, the AES-256 at-rest
          encryption key, and the {envFile} file:
        </Typography>
        <CommandField
          value={setupCommand}
          copyLabel="Setup command"
          onCopy={onCopy}
          colorScheme={colorScheme}
        />
      </Box>

      {/* Step 3: Configure the Gateway */}
      <Box>
        <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
          Step 3: Configure the Gateway
        </Typography>
        {registrationToken ? (
          <>
            {hasJustRegeneratedToken && (
              <Alert severity="success" sx={{ mb: 2 }}>
                Successfully generated new configurations. Use the updated
                command below to reconfigure your gateway.
              </Alert>
            )}
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Run this command to add the control plane connection settings to{" "}
              {envFile}:
            </Typography>
            <CommandField
              value={buildConfigureCommand("<your-gateway-token>")}
              copyValue={buildConfigureCommand(registrationToken)}
              copyLabel="Configure command"
              onCopy={onCopy}
              colorScheme={colorScheme}
              minRows={4}
            />
            <Alert severity="info" sx={{ mt: 2 }}>
              To gain gateway analytics, you can integrate with Moesif by adding
              your Moesif application token with the key <code>MOESIF_KEY</code>{" "}
              to your <code>{envFile}</code>.
            </Alert>
          </>
        ) : (
          <ReconfigurePrompt
            isRegeneratingToken={isRegeneratingToken}
            onRegenerateToken={onRegenerateToken}
          />
        )}
      </Box>

      {/* Step 4: Start the Gateway */}
      <Box>
        <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
          Step 4: Start the Gateway
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ my: 2 }}>
          From the gateway folder, run this command to start the gateway using
          the {envFile} file:
        </Typography>
        <CommandField
          value={startCommand}
          copyLabel="Start command"
          onCopy={onCopy}
          colorScheme={colorScheme}
        />
        {renderConnectionStatus()}
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
  registrationToken,
  hasJustRegeneratedToken,
  isRegeneratingToken,
  onRegenerateToken,
  onCopy,
  colorScheme,
  renderConnectionStatus,
}: GatewaySetupStepsProps) {
  const envFile = `${gatewayFolderName}/configs/keys.env`;
  const buildConfigureCommand = (token: string) =>
    `cat > ${envFile} << 'ENVFILE'
GATEWAY_CONTROLPLANE_HOST=${CONTROLPLANE_HOST}
GATEWAY_REGISTRATION_TOKEN=${token}
ENVFILE`;
  const navigateCommand = `cd ${gatewayFolderName}`;
  const startCommand = "docker compose --env-file configs/keys.env up";

  return (
    <>
      <DownloadStep
        gatewayVersion={gatewayVersion}
        gatewayZipName={gatewayZipName}
        onCopy={onCopy}
        colorScheme={colorScheme}
      />

      {/* Step 2: Configure the Gateway */}
      <Box>
        <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
          Step 2: Configure the Gateway
        </Typography>
        {registrationToken ? (
          <>
            {hasJustRegeneratedToken && (
              <Alert severity="success" sx={{ mb: 2 }}>
                Successfully generated new configurations. Use the updated
                command below to reconfigure your gateway.
              </Alert>
            )}
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Run this command to create {envFile} with the required environment
              variables:
            </Typography>
            <CommandField
              value={buildConfigureCommand("<your-gateway-token>")}
              copyValue={buildConfigureCommand(registrationToken)}
              copyLabel="Configure command"
              onCopy={onCopy}
              colorScheme={colorScheme}
              minRows={4}
            />
            <Alert severity="info" sx={{ mt: 2 }}>
              To gain gateway analytics, you can integrate with Moesif by adding
              your Moesif application token with the key <code>MOESIF_KEY</code>{" "}
              to your <code>configs/keys.env</code>.
            </Alert>
          </>
        ) : (
          <ReconfigurePrompt
            isRegeneratingToken={isRegeneratingToken}
            onRegenerateToken={onRegenerateToken}
          />
        )}
      </Box>

      {/* Step 3: Start the Gateway */}
      <Box>
        <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
          Step 3: Start the Gateway
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
          1. Navigate to the gateway folder.
        </Typography>
        <CommandField
          value={navigateCommand}
          copyLabel="Navigate command"
          onCopy={onCopy}
          colorScheme={colorScheme}
        />
        <Typography variant="body2" color="text.secondary" sx={{ my: 2 }}>
          2. Run this command to start the gateway using the configs/keys.env
          file created in Step 2:
        </Typography>
        <CommandField
          value={startCommand}
          copyLabel="Start command"
          onCopy={onCopy}
          colorScheme={colorScheme}
        />
        {renderConnectionStatus()}
      </Box>
    </>
  );
}
