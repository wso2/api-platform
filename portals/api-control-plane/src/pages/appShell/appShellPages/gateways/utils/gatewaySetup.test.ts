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

import { describe, expect, it } from 'vitest';

import type { GatewayVersionEntry } from '@/config/runtime';
import { aGateway } from '@/test/msw';
import {
  configureCommand,
  downloadCommand,
  helmInstallCommand,
  setupScriptCommand,
  setupTarget,
  startCommand,
  usesSetupScript,
} from './gatewaySetup';

const HOST = 'connect.example.com';

/** Mirrors a real deployment's `[[gateway_versions]]`, newest first per type. */
const CATALOG: GatewayVersionEntry[] = [
  { channel: 'LTS', latestVersion: 'v1.2.0', type: 'regular', version: '1.2' },
  { channel: 'LTS', latestVersion: 'v1.1.0', type: 'regular', version: '1.1' },
  { channel: 'LTS', latestVersion: 'v1.0.0', type: 'event', version: '1.0' },
];

/**
 * These strings are pasted into a user's terminal, so the assertions are on the
 * exact text rather than on "contains something plausible". Release paths and
 * archive names match what github.com/wso2/api-platform actually publishes
 * (e.g. `gateway/v1.2.0/wso2apip-api-gateway-1.2.0.zip`).
 */
describe('gatewaySetup', () => {
  it('resolves a configured release to its published tag and archive', () => {
    expect(
      setupTarget(aGateway({ functionalityType: 'regular', version: '1.2' }), HOST, CATALOG),
    ).toEqual({
      controlPlaneHost: HOST,
      distribution: 'wso2apip-api-gateway-1.2.0',
      flow: 'setupScript',
      releaseTag: 'gateway/v1.2.0',
      version: '1.2.0',
    });

    expect(
      setupTarget(aGateway({ functionalityType: 'event', version: '1.0' }), HOST, CATALOG),
    ).toMatchObject({
      distribution: 'wso2apip-event-gateway-1.0.0',
      flow: 'envFile',
      releaseTag: 'event-gateway/v1.0.0',
    });
  });

  it('names the archive after distVersion when the catalog sets one', () => {
    const catalog: GatewayVersionEntry[] = [
      {
        channel: 'LTS',
        distVersion: '1.2.0-rc',
        latestVersion: 'v1.2.0',
        type: 'regular',
        version: '1.2',
      },
    ];

    expect(setupTarget(aGateway({ version: '1.2' }), HOST, catalog)).toMatchObject({
      distribution: 'wso2apip-api-gateway-1.2.0-rc',
      releaseTag: 'gateway/v1.2.0',
      version: '1.2.0',
    });
  });

  it('keeps the version of a gateway whose release the catalog no longer lists', () => {
    // Offering another release would hand out a build the gateway is not running.
    // The stored `major.minor` names that minor's first published release, since
    // no release is tagged or archived without a patch number.
    expect(setupTarget(aGateway({ version: '0.9' }), HOST, CATALOG)).toMatchObject({
      distribution: 'wso2apip-api-gateway-0.9.0',
      releaseTag: 'gateway/v0.9.0',
      version: '0.9.0',
    });
  });

  it('keeps a full or date-based version the catalog no longer lists as it is', () => {
    expect(setupTarget(aGateway({ version: '2026.09.24' }), HOST, CATALOG)).toMatchObject({
      distribution: 'wso2apip-api-gateway-2026.09.24',
      releaseTag: 'gateway/v2026.09.24',
    });
  });

  it('takes the type’s latest release for a gateway registered without a version', () => {
    expect(setupTarget(aGateway({ version: undefined }), HOST, CATALOG)).toMatchObject({
      releaseTag: 'gateway/v1.2.0',
    });
  });

  it('falls back to a published release when nothing else is known', () => {
    // `version` is optional in the spec, and a download URL built from
    // `undefined` would 404 rather than fail visibly here.
    const target = setupTarget(aGateway({ version: undefined }), HOST, []);

    expect(target.releaseTag).toBe('gateway/v1.0.0');
    expect(downloadCommand(target)).not.toContain('undefined');
  });

  it('builds a download URL from the release tag and archive name', () => {
    const target = setupTarget(aGateway({ version: '1.2' }), HOST, CATALOG);

    expect(downloadCommand(target)).toBe(
      'curl -sLO https://github.com/wso2/api-platform/releases/download/gateway/v1.2.0/wso2apip-api-gateway-1.2.0.zip && \\\n' +
        'unzip wso2apip-api-gateway-1.2.0.zip',
    );
  });

  it('uses the setup script and api-platform.env from 1.2 on', () => {
    const target = setupTarget(aGateway({ version: '1.2' }), HOST, CATALOG);

    expect(setupScriptCommand(target)).toBe('./wso2apip-api-gateway-1.2.0/scripts/setup.sh');
    expect(configureCommand(target, 'secret-token')).toBe(
      "cat >> wso2apip-api-gateway-1.2.0/api-platform.env << 'ENVFILE'\n" +
        `APIP_GW_CONTROLLER_CONTROLPLANE_HOST=${HOST}:443\n` +
        'APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN=secret-token\n' +
        'ENVFILE',
    );
    expect(startCommand(target)).toBe('docker compose up');
  });

  it('keeps an explicit control plane port or path in the api-platform.env host', () => {
    const withPort = setupTarget(aGateway({ version: '1.2' }), 'localhost:9243', CATALOG);
    expect(configureCommand(withPort, 'secret-token')).toContain(
      'APIP_GW_CONTROLLER_CONTROLPLANE_HOST=localhost:9243\n',
    );

    // WSO2 Cloud routes the control plane through a gateway path; a port
    // appended after it would point at a different route.
    const routed = 'dev-wso2cloud.gateway.example.com/apip-platform-api-internal-endpoint';
    const withPath = setupTarget(aGateway({ version: '1.2' }), routed, CATALOG);
    expect(configureCommand(withPath, 'secret-token')).toContain(
      `APIP_GW_CONTROLLER_CONTROLPLANE_HOST=${routed}\n`,
    );
  });

  it('writes configs/keys.env for releases before 1.2', () => {
    const target = setupTarget(aGateway({ version: '1.1' }), HOST, CATALOG);
    const command = configureCommand(target, 'secret-token');

    expect(target.flow).toBe('envFile');
    expect(command).toContain('wso2apip-api-gateway-1.1.0/configs/keys.env');
    expect(command).toContain(`GATEWAY_CONTROLPLANE_HOST=${HOST}`);
    expect(command).toContain('GATEWAY_REGISTRATION_TOKEN=secret-token');
    expect(startCommand(target)).toBe('docker compose --env-file configs/keys.env up');
  });

  it('installs the Helm chart at the full release version, with the token bound', () => {
    const target = setupTarget(aGateway({ version: '1.2' }), HOST, CATALOG);
    const command = helmInstallCommand(target, 'edge-gateway', 'secret-token');

    expect(command).toContain('helm install edge-gateway ');
    expect(command).toContain('--version 1.2.0');
    expect(command).toContain(`controlPlane.host="${HOST}"`);
    expect(command).toContain('token.value="secret-token"');
  });

  it('treats 1.2 and later, including date-versioned releases, as the setup-script flow', () => {
    expect(usesSetupScript('1.1.0')).toBe(false);
    expect(usesSetupScript('1.2.0')).toBe(true);
    expect(usesSetupScript('2.0.0')).toBe(true);
    expect(usesSetupScript('2026.09.24')).toBe(true);
    expect(usesSetupScript('not-a-version')).toBe(true);
  });
});
