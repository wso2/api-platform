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

import { useState, type ReactNode } from 'react';
import { Alert, Box, Button, Typography } from '@wso2/oxygen-ui';
import { Check, Copy } from '@wso2/oxygen-ui-icons-react';

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
  /** The release path segment: `releases/download/<releaseChannel>/<version>/...`. */
  releaseChannel: string;
  /** `host:port` the gateway's controller dials to reach the control plane. */
  controlPlaneHost: string;
  registrationToken: string | null;
  hasJustRegeneratedToken: boolean;
  isRegeneratingToken: boolean;
  onRegenerateToken: () => void;
};

const ACCENT = '#f7882f';

/** A read-only command block: a "bash" header with a Copy button, then the command behind a `$` prompt. */
export function CommandField({
  value,
  copyValue,
  copyLabel,
}: {
  value: string;
  /** Text placed on the clipboard when copied; defaults to `value`. */
  copyValue?: string;
  copyLabel: string;
}) {
  const [copied, setCopied] = useState(false);
  const lines = value.split('\n');

  const handleCopy = () => {
    void navigator.clipboard.writeText(copyValue ?? value).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };

  return (
    <Box sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '12px', overflow: 'hidden' }}>
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          px: 2,
          py: 0.75,
          borderBottom: '1px solid',
          borderColor: 'divider',
        }}
      >
        <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
          bash
        </Typography>
        <Button
          size="small"
          aria-label={`Copy ${copyLabel}`}
          onClick={handleCopy}
          startIcon={copied ? <Check size={14} /> : <Copy size={14} />}
          sx={{ textTransform: 'none', color: 'text.secondary', minWidth: 0 }}
        >
          {copied ? 'Copied' : 'Copy'}
        </Button>
      </Box>
      <Box
        component="pre"
        sx={{
          m: 0,
          p: 2,
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
          fontSize: '0.875rem',
          lineHeight: 1.7,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
          bgcolor: 'action.hover',
        }}
      >
        {lines.map((line, index) => (
          <Box component="span" key={index} sx={{ display: 'block' }}>
            {index === 0 ? (
              <Box component="span" sx={{ color: ACCENT, mr: 1 }}>
                $
              </Box>
            ) : null}
            {line}
          </Box>
        ))}
      </Box>
    </Box>
  );
}

/** One numbered step; a vertical line joins it to the next unless it is the last. */
export function Step({
  number,
  title,
  last,
  children,
}: {
  number: number;
  title: string;
  last?: boolean;
  children: ReactNode;
}) {
  return (
    <Box sx={{ display: 'flex', gap: 2 }}>
      <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', flexShrink: 0 }}>
        <Box
          sx={{
            width: 40,
            height: 40,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            borderRadius: '50%',
            border: `1.5px solid ${ACCENT}`,
            color: 'text.primary',
            fontWeight: 500,
          }}
        >
          {number}
        </Box>
        {last ? null : <Box sx={{ flex: 1, width: '1px', mt: 1, bgcolor: 'divider' }} />}
      </Box>
      <Box sx={{ flex: 1, minWidth: 0, pb: last ? 0 : 4 }}>
        <Typography variant="h6" sx={{ fontWeight: 600, lineHeight: '40px', mb: 0.5 }}>
          {title}
        </Typography>
        {children}
      </Box>
    </Box>
  );
}

export const StepText = ({ children }: { children: ReactNode }) => (
  <Typography variant="body1" color="text.secondary" sx={{ mb: 2 }}>
    {children}
  </Typography>
);

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
    <>
      <StepText>Run this command in your terminal to download the gateway:</StepText>
      <CommandField value={downloadCommand} copyLabel="Download command" />
    </>
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
      <StepText>
        Your existing local gateway can still use the previously generated token. If you want to generate a new token
        and new configuration command, click Reconfigure. This will revoke the previous token.
      </StepText>
      <Button variant="outlined" color="primary" onClick={onRegenerateToken} disabled={isRegeneratingToken}>
        Reconfigure
      </Button>
    </>
  );
}

/** The success note shown right after a token is regenerated. */
const RegeneratedNote = () => (
  <Alert severity="success" sx={{ mb: 2 }}>
    Successfully generated new configurations. Use the updated command below to reconfigure your gateway.
  </Alert>
);

/**
 * Setup steps for gateway v1.2 and above:
 * 1. Download → 2. Set up (setup.sh) → 3. Configure (api-platform.env) → 4. Start.
 */
export function GatewaySetupStepsV1_2Plus({
  gatewayVersion,
  gatewayZipName,
  gatewayFolderName,
  releaseChannel,
  controlPlaneHost,
  registrationToken,
  hasJustRegeneratedToken,
  isRegeneratingToken,
  onRegenerateToken,
}: GatewaySetupStepsProps): ReactNode {
  const envFile = 'api-platform.env';
  const setupCommand = `cd ${gatewayFolderName} && ./scripts/setup.sh`;
  const buildConfigureCommand = (token: string) =>
    `cat >> ${envFile} << 'ENVFILE'
APIP_GW_CONTROLLER_CONTROLPLANE_HOST=${controlPlaneHost}
APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN=${token}
ENVFILE`;
  const startCommand = 'docker compose up';

  return (
    <Box>
      <Step number={1} title="Download the Gateway">
        <DownloadStep gatewayVersion={gatewayVersion} gatewayZipName={gatewayZipName} releaseChannel={releaseChannel} />
      </Step>

      <Step number={2} title="Set up the Gateway">
        <StepText>
          Navigate into the gateway folder and run the setup script. It provisions the listener TLS certificate, the
          AES-256 at-rest encryption key, and the {envFile} file:
        </StepText>
        <CommandField value={setupCommand} copyLabel="Setup command" />
      </Step>

      <Step number={3} title="Configure the Gateway">
        {registrationToken ? (
          <>
            {hasJustRegeneratedToken && <RegeneratedNote />}
            <StepText>Run this command to add the control plane connection settings to {envFile}:</StepText>
            <CommandField
              value={buildConfigureCommand('<your-gateway-token>')}
              copyValue={buildConfigureCommand(registrationToken)}
              copyLabel="Configure command"
            />
          </>
        ) : (
          <ReconfigurePrompt isRegeneratingToken={isRegeneratingToken} onRegenerateToken={onRegenerateToken} />
        )}
      </Step>

      <Step number={4} title="Start the Gateway" last>
        <StepText>From the gateway folder, run this command to start the gateway using the {envFile} file:</StepText>
        <CommandField value={startCommand} copyLabel="Start command" />
      </Step>
    </Box>
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
  controlPlaneHost,
  registrationToken,
  hasJustRegeneratedToken,
  isRegeneratingToken,
  onRegenerateToken,
}: GatewaySetupStepsProps): ReactNode {
  const envFile = `${gatewayFolderName}/configs/keys.env`;
  const buildConfigureCommand = (token: string) =>
    `cat > ${envFile} << 'ENVFILE'
GATEWAY_CONTROLPLANE_HOST=${controlPlaneHost}
GATEWAY_REGISTRATION_TOKEN=${token}
ENVFILE`;
  const navigateCommand = `cd ${gatewayFolderName}`;
  const startCommand = 'docker compose --env-file configs/keys.env up';

  return (
    <Box>
      <Step number={1} title="Download the Gateway">
        <DownloadStep gatewayVersion={gatewayVersion} gatewayZipName={gatewayZipName} releaseChannel={releaseChannel} />
      </Step>

      <Step number={2} title="Configure the Gateway">
        {registrationToken ? (
          <>
            {hasJustRegeneratedToken && <RegeneratedNote />}
            <StepText>Run this command to create {envFile} with the required environment variables:</StepText>
            <CommandField
              value={buildConfigureCommand('<your-gateway-token>')}
              copyValue={buildConfigureCommand(registrationToken)}
              copyLabel="Configure command"
            />
          </>
        ) : (
          <ReconfigurePrompt isRegeneratingToken={isRegeneratingToken} onRegenerateToken={onRegenerateToken} />
        )}
      </Step>

      <Step number={3} title="Start the Gateway" last>
        <StepText>1. Navigate to the gateway folder.</StepText>
        <CommandField value={navigateCommand} copyLabel="Navigate command" />
        <Box sx={{ mt: 2 }}>
          <StepText>
            2. Run this command to start the gateway using the configs/keys.env file created in Step 2:
          </StepText>
        </Box>
        <CommandField value={startCommand} copyLabel="Start command" />
      </Step>
    </Box>
  );
}

export type GatewaySetupStepsKubernetesProps = {
  /** The chart version: the release without a leading "v". */
  gatewayVersionHelm: string;
  /** Names the Helm release, so several gateways can share a cluster. */
  releaseName: string;
  controlPlaneHost: string;
  registrationToken: string | null;
  hasJustRegeneratedToken: boolean;
  isRegeneratingToken: boolean;
  onRegenerateToken: () => void;
};

const HELM_CHART_REF = 'oci://ghcr.io/wso2/api-platform/helm-charts/gateway';

/** Kubernetes: one step, the Helm install, which carries the control plane host and token. */
export function GatewaySetupStepsKubernetes({
  gatewayVersionHelm,
  releaseName,
  controlPlaneHost,
  registrationToken,
  hasJustRegeneratedToken,
  isRegeneratingToken,
  onRegenerateToken,
}: GatewaySetupStepsKubernetesProps): ReactNode {
  const buildInstallCommand = (token: string) =>
    `helm install ${releaseName} ${HELM_CHART_REF} --version ${gatewayVersionHelm} \\
  --set gateway.controller.controlPlane.host="${controlPlaneHost}" \\
  --set gateway.controller.controlPlane.port=443 \\
  --set gateway.controller.controlPlane.token.value="${token}"`;

  return (
    <Box>
      <Step number={1} title="Install the Chart" last>
        {registrationToken ? (
          <>
            {hasJustRegeneratedToken && <RegeneratedNote />}
            <StepText>Run this command to install the gateway chart with control plane configurations:</StepText>
            <CommandField
              value={buildInstallCommand('your-gateway-token')}
              copyValue={buildInstallCommand(registrationToken)}
              copyLabel="Helm install command"
            />
          </>
        ) : (
          <ReconfigurePrompt isRegeneratingToken={isRegeneratingToken} onRegenerateToken={onRegenerateToken} />
        )}
      </Step>
    </Box>
  );
}
