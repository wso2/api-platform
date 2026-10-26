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
 * Port the Helm chart's agent connects to the control plane on. The Docker
 * path doesn't use it: there, `controlPlaneHost` carries `host:port` whole.
 */
const CONTROL_PLANE_PORT = 443;

/** Where release archives are published. */
const RELEASE_BASE = 'https://github.com/wso2/api-platform/releases/download';

/** OCI registry holding the gateway Helm chart. */
const HELM_CHART_REF = 'oci://ghcr.io/wso2/api-platform/helm-charts/gateway';

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

/**
 * Release tag prefix per distribution. The regular gateway is tagged
 * `gateway/v…` even though its archive is named `wso2apip-api-gateway-…`
 * (gateway-release.yml), so the tag can't be derived from the archive name.
 */
const RELEASE_TRACK: Record<GatewayFunctionality, string> = {
  ai: 'ai-gateway',
  event: 'event-gateway',
  regular: 'gateway',
};

/**
 * The newest published release on each track, used whenever a gateway's
 * `version` names a product line (the create form offers "1.0") rather than a
 * release that exists. "1.0" has never been a tag on any track, so a URL built
 * from it 404s.
 */
const CURRENT_RELEASE: Record<GatewayFunctionality, string> = {
  ai: '2026.09.24',
  event: '0.9.0',
  regular: '2026.09.24',
};

/** A published release number: `1.2.0`, or a date release such as `2026.09.24`. */
const RELEASE_VERSION = /^\d+\.\d+\.\d+$/;

/**
 * Hosts that mean "this machine" to a browser but "this container" to a
 * gateway running in Docker. Inside the container they have to be
 * `host.docker.internal`, which the distribution's compose file maps to the
 * host on every platform.
 */
const LOOPBACK_HOST = /^(localhost|127\.0\.0\.1)(?=:|$)/;

/** Everything the command builders need about the gateway being set up. */
export type GatewaySetupTarget = {
  /** Control plane host the agent registers against. */
  controlPlaneHost: string;
  /** Archive/directory name, without the `.zip` suffix. */
  distribution: string;
  /**
   * Whether the distribution ships `scripts/setup.sh` and reads its settings
   * from `api-platform.env` (the API and AI gateways). The event gateway has no
   * setup script and takes its settings as compose variables instead.
   */
  hasSetupScript: boolean;
  /** Release tag segment, e.g. `gateway/v2026.09.24`. */
  releaseTag: string;
  /** The release version, e.g. `2026.09.24`. */
  version: string;
};

/**
 * Resolves a gateway plus the deployment's control plane host into the values
 * every command below is built from.
 */
export const setupTarget = (gateway: Gateway, controlPlaneHost: string): GatewaySetupTarget => {
  const functionality = gateway.functionalityType ?? 'regular';
  const requested = gateway.version?.trim() ?? '';
  const version = RELEASE_VERSION.test(requested) ? requested : CURRENT_RELEASE[functionality];

  return {
    controlPlaneHost,
    distribution: `wso2apip-${DISTRIBUTION[functionality]}-${version}`,
    hasSetupScript: functionality !== 'event',
    releaseTag: `${RELEASE_TRACK[functionality]}/v${version}`,
    version,
  };
};

/**
 * The control plane host as a Docker-hosted gateway must address it: a
 * loopback host becomes `host.docker.internal`, keeping the port.
 */
export const dockerControlPlaneHost = (host: string): string =>
  host.replace(LOOPBACK_HOST, 'host.docker.internal');

/**
 * Fetches and unpacks the release archive. `-f` makes a missing release fail
 * with an HTTP error; without it curl saves the 404 page as the zip and the
 * failure surfaces later as a confusing unzip error.
 */
export const downloadCommand = (target: GatewaySetupTarget): string =>
  `curl -fLO ${RELEASE_BASE}/${target.releaseTag}/${target.distribution}.zip && \\\n` +
  `unzip ${target.distribution}.zip`;

/**
 * Enters the unpacked distribution and, where the distribution has one, runs
 * its one-time setup, which creates the certificates, keys and
 * `api-platform.env` file the gateway starts from. The configure step appends
 * to that file, so setup has to come first.
 */
export const prepareCommand = (target: GatewaySetupTarget): string =>
  target.hasSetupScript
    ? `cd ${target.distribution} && ./scripts/setup.sh`
    : `cd ${target.distribution}`;

/**
 * Hands the gateway its control plane host and registration token, from
 * inside the distribution folder.
 *
 * API and AI gateways: appended to `api-platform.env`, the file their
 * containers read (the two lines setup.sh asks to be added by hand). Appending
 * keeps setup's own values.
 *
 * Event gateway: written to `configs/keys.env`, which `docker compose
 * --env-file` feeds into the compose file's `${GATEWAY_…}` variables.
 *
 * A heredoc rather than `echo` lines so the whole block is one paste, and the
 * token never lands in the user's shell history as a bare argument.
 */
export const configureCommand = (target: GatewaySetupTarget, token: string): string => {
  const host = dockerControlPlaneHost(target.controlPlaneHost);

  return target.hasSetupScript
    ? `cat >> api-platform.env << 'ENVFILE'\n` +
        `APIP_GW_CONTROLLER_CONTROLPLANE_HOST=${host}\n` +
        `APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN=${token}\n` +
        `ENVFILE`
    : `cat > configs/keys.env << 'ENVFILE'\n` +
        `GATEWAY_CONTROLPLANE_HOST=${host}\n` +
        `GATEWAY_REGISTRATION_TOKEN=${token}\n` +
        `ENVFILE`;
};

/** Brings the gateway up, from inside the distribution folder. */
export const startCommand = (target: GatewaySetupTarget): string =>
  target.hasSetupScript ? 'docker compose up' : 'docker compose --env-file configs/keys.env up';

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
