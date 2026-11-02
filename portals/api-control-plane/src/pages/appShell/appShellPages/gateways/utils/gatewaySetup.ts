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
import type { GatewayVersionEntry } from '@/config/runtime';
import type { GatewayFunctionality } from './gatewayDisplay';
import { findVersionEntry } from './gatewayVersions';

/**
 * The shell commands shown in a gateway's "Get Started" panel.
 *
 * Kept as pure string builders, outside the components that render them, for
 * one reason: these are the instructions a user pastes into a terminal, and a
 * wrong one fails on their machine rather than in ours. As functions they are
 * unit-testable against the gateway shapes the spec allows; embedded in JSX
 * they would only ever be checked by eye.
 *
 * Every value that varies: the release catalog, the control plane host, the
 * token, is an argument. Nothing here reads config or state.
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

/** Fallback release tag when no gateway version is available. */
const FALLBACK_RELEASE_TAG = 'v1.0.0';

/** Env file the setup script writes and `docker compose` reads (gateway 1.2+). */
const SETUP_SCRIPT_ENV_FILE = 'api-platform.env';

/**
 * Archive name each functionality type ships under:
 * `wso2apip-<name>-<version>.zip`, unpacking into a folder of the same name.
 */
const DISTRIBUTION: Record<GatewayFunctionality, string> = {
  ai: 'ai-gateway',
  event: 'event-gateway',
  regular: 'api-gateway',
};

/** The distribution name a functionality type ships under, e.g. `api-gateway`. */
export const distributionName = (type: GatewayFunctionality): string => DISTRIBUTION[type];

/**
 * GitHub release path each type is tagged under:
 * `releases/download/<path>/<tag>/`. The regular gateway's releases are tagged
 * `gateway/v…`, not after its archive name.
 */
const RELEASE_PATH: Record<GatewayFunctionality, string> = {
  ai: 'ai-gateway',
  event: 'event-gateway',
  regular: 'gateway',
};

/** How a release is brought up based on its version. */
export type GatewaySetupFlow = 'envFile' | 'setupScript';

/** Everything the command builders need about the gateway being set up. */
export type GatewaySetupTarget = {
  /** Control plane host the agent registers against. */
  controlPlaneHost: string;
  /** Archive/directory name, without the `.zip` suffix. */
  distribution: string;
  flow: GatewaySetupFlow;
  /** Release path and tag, e.g. `gateway/v1.2.0`. */
  releaseTag: string;
  /** Full release version, e.g. `1.2.0` — also the Helm chart version. */
  version: string;
};

/**
 * Whether a release uses the setup-script flow (1.2 and later). A version that
 * does not parse as `major.minor` is treated as current, as is a date-versioned
 * release (e.g. `2026.09.24`).
 */
export const usesSetupScript = (version: string): boolean => {
  const [major, minor] = version.split('.').map((part) => Number.parseInt(part, 10));
  if (Number.isNaN(major) || Number.isNaN(minor)) return true;
  return major > 1 || (major === 1 && minor >= 2);
};

/**
 * Resolves a gateway, the deployment's release catalog and its control plane
 * host into the values every command below is built from.
 *
 * A gateway whose version the catalog does not list (a release since removed
 * from config) keeps its own version rather than borrowing another release.
 */
export const setupTarget = (
  gateway: Gateway,
  controlPlaneHost: string,
  catalog: readonly GatewayVersionEntry[],
): GatewaySetupTarget => {
  const type = gateway.functionalityType ?? 'regular';
  const reported = gateway.version?.trim() || undefined;
  const entry = findVersionEntry(catalog, type, reported);

  const knownVersion = entry?.version ?? reported;
  const tag = entry?.latestVersion ?? (knownVersion ? `v${knownVersion}` : FALLBACK_RELEASE_TAG);
  const version = tag.replace(/^v/, '');
  const distVersion = entry?.distVersion ?? version;

  return {
    controlPlaneHost,
    distribution: `wso2apip-${DISTRIBUTION[type]}-${distVersion}`,
    flow: usesSetupScript(version) ? 'setupScript' : 'envFile',
    releaseTag: `${RELEASE_PATH[type]}/${tag}`,
    version,
  };
};

/** Adds the default HTTPS port to a bare control plane hostname. */
const controlPlaneAddress = (host: string): string =>
  /:\d+$/.test(host) || host.includes('/') ? host : `${host}:${CONTROL_PLANE_PORT}`;

/** Fetches and unpacks the release archive. */
export const downloadCommand = (target: GatewaySetupTarget): string =>
  `curl -sLO ${RELEASE_BASE}/${target.releaseTag}/${target.distribution}.zip && \\\n` +
  `unzip ${target.distribution}.zip`;

/** Runs the setup script to generate certificates, keys, and `api-platform.env`. */
export const setupScriptCommand = (target: GatewaySetupTarget): string =>
  `cd ${target.distribution} && ./scripts/setup.sh`;

/**
 * Writes the registration token and control plane host into the env file the
 * gateway reads at start-up.
 *
 * A heredoc rather than separate `echo` lines so the whole file is one paste,
 * and the token never lands in the user's shell history as a bare argument.
 * The setup-script flow appends to the `api-platform.env` that `setup.sh`
 * wrote, from inside the gateway folder; the older flow writes a fresh
 * `configs/keys.env`.
 */
export const configureCommand = (target: GatewaySetupTarget, token: string): string =>
  target.flow === 'setupScript'
    ? `cat >> ${SETUP_SCRIPT_ENV_FILE} << 'ENVFILE'\n` +
      `APIP_GW_CONTROLLER_CONTROLPLANE_HOST=${controlPlaneAddress(target.controlPlaneHost)}\n` +
      `APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN=${token}\n` +
      `ENVFILE`
    : `cat > ${target.distribution}/configs/keys.env << 'ENVFILE'\n` +
      `GATEWAY_CONTROLPLANE_HOST=${target.controlPlaneHost}\n` +
      `GATEWAY_CONTROLPLANE_PORT=${CONTROL_PLANE_PORT}\n` +
      `GATEWAY_REGISTRATION_TOKEN=${token}\n` +
      `ENVFILE`;

/** Enters the unpacked distribution. */
export const enterDirectoryCommand = (target: GatewaySetupTarget): string =>
  `cd ${target.distribution}`;

/** Brings the gateway up with the env file written in the configure step. */
export const startCommand = (target: GatewaySetupTarget): string =>
  target.flow === 'setupScript'
    ? 'docker compose up'
    : 'docker compose --env-file configs/keys.env up';

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
