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

import type { Gateway } from '@/api/resources/gateways';
import type { GatewayFunctionality } from './gatewayDisplay';

/**
 * The shell commands shown in a gateway's "Get Started" panel.
 *
 * Kept as pure string builders, outside the components that render them, for
 * one reason: these are the instructions a user pastes into a terminal, and a
 * wrong one fails on their machine rather than in ours. As functions they are
 * unit-testable against the gateway shapes the spec allows; embedded in JSX
 * they would only ever be checked by eye.
 *
 * Every value that varies: the distribution name, the version, the control
 * plane host, the token, is an argument. Nothing here reads config or state.
 */

/** Placeholder standing in for a token the user has not generated yet. */
export const TOKEN_PLACEHOLDER = '<your-gateway-token>';

/**
 * Port the gateway agent connects to the control plane on. Fixed rather than
 * configurable: the control plane is reached over TLS on the standard HTTPS
 * port, and `gatewayControlPlaneHost` carries the only part that varies.
 */
const CONTROL_PLANE_PORT = 443;

/** Where release archives are published. */
const RELEASE_BASE = 'https://github.com/wso2/api-platform/releases/download';

/** OCI registry holding the gateway Helm chart. */
const HELM_CHART_REF = 'oci://ghcr.io/wso2/api-platform/helm-charts/gateway';

/** Version assumed when a gateway predates the `version` field. */
const FALLBACK_VERSION = '1.0';

/**
 * Which distribution serves this gateway's traffic. The three functionality
 * types ship as three separate archives and three separate release tracks, so
 * the download URL follows the type rather than being one artifact for all.
 */
const DISTRIBUTION: Record<GatewayFunctionality, string> = {
  ai: 'ai-gateway',
  event: 'event-gateway',
  regular: 'api-gateway',
};

/** Everything the command builders need about the gateway being set up. */
export type GatewaySetupTarget = {
  /** Control plane host the agent registers against. */
  controlPlaneHost: string;
  /** Archive/directory name, without the `.zip` suffix. */
  distribution: string;
  /** Release tag segment, e.g. `v1.0`. */
  releaseTag: string;
  /** The gateway's own release version, e.g. `1.0`. */
  version: string;
};

/**
 * Resolves a gateway plus the deployment's control plane host into the values
 * every command below is built from.
 */
export const setupTarget = (gateway: Gateway, controlPlaneHost: string): GatewaySetupTarget => {
  const version = gateway.version?.trim() || FALLBACK_VERSION;
  const family = DISTRIBUTION[gateway.functionalityType ?? 'regular'];

  return {
    controlPlaneHost,
    distribution: `wso2apip-${family}-${version}`,
    releaseTag: `${family}/v${version}`,
    version,
  };
};

/** Fetches and unpacks the release archive. */
export const downloadCommand = (target: GatewaySetupTarget): string =>
  `curl -sLO ${RELEASE_BASE}/${target.releaseTag}/${target.distribution}.zip && \\\n` +
  `unzip ${target.distribution}.zip`;

/**
 * Writes the registration token and control plane host into the env file the
 * gateway reads at start-up.
 *
 * A heredoc rather than two `echo` lines so the whole file is one paste, and
 * the token never lands in the user's shell history as a bare argument.
 */
export const configureCommand = (target: GatewaySetupTarget, token: string): string =>
  `cat > ${target.distribution}/configs/keys.env << 'ENVFILE'\n` +
  `GATEWAY_CONTROLPLANE_HOST=${target.controlPlaneHost}\n` +
  `GATEWAY_CONTROLPLANE_PORT=${CONTROL_PLANE_PORT}\n` +
  `GATEWAY_REGISTRATION_TOKEN=${token}\n` +
  `ENVFILE`;

/** Enters the unpacked distribution. */
export const enterDirectoryCommand = (target: GatewaySetupTarget): string =>
  `cd ${target.distribution}`;

/** Brings the gateway up with the env file written in the configure step. */
export const startCommand = (): string => 'docker compose --env-file configs/keys.env up';

/** Confirms the container runtime a virtual-machine install depends on. */
export const runtimeCheckCommand = (): string => 'docker --version\ndocker compose version';

/**
 * Installs the gateway chart, pointed at this control plane.
 *
 * The release name is the gateway's own handle so a cluster running several
 * gateways keeps them apart, and so the name in the command matches the name
 * in this console.
 */
export const helmInstallCommand = (
  target: GatewaySetupTarget,
  releaseName: string,
  token: string,
): string =>
  `helm install ${releaseName} ${HELM_CHART_REF} --version ${target.version} \\\n` +
  `  --set gateway.controller.controlPlane.host="${target.controlPlaneHost}" \\\n` +
  `  --set gateway.controller.controlPlane.port=${CONTROL_PLANE_PORT} \\\n` +
  `  --set gateway.controller.controlPlane.token.value="${token}"`;
