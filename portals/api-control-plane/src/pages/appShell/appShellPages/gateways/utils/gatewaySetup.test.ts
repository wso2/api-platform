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

import { aGateway } from '@/test/msw';
import {
  configureCommand,
  dockerControlPlaneHost,
  downloadCommand,
  helmInstallCommand,
  prepareCommand,
  setupTarget,
  startCommand,
} from './gatewaySetup';

const HOST = 'connect.example.com';

/**
 * These strings are pasted into a user's terminal, so the assertions are on the
 * exact text rather than on "contains something plausible".
 */
describe('gatewaySetup', () => {
  it('names the distribution after the gateway functionality and release', () => {
    expect(
      setupTarget(aGateway({ functionalityType: 'ai', version: '1.1.0' }), HOST),
    ).toMatchObject({
      distribution: 'wso2apip-ai-gateway-1.1.0',
      releaseTag: 'ai-gateway/v1.1.0',
      version: '1.1.0',
    });

    expect(
      setupTarget(aGateway({ functionalityType: 'event', version: '0.9.0' }), HOST),
    ).toMatchObject({ distribution: 'wso2apip-event-gateway-0.9.0' });
  });

  it('tags the regular gateway on the gateway/ track, not api-gateway/', () => {
    // The archive is wso2apip-api-gateway-*, but the release tag is gateway/v*.
    const target = setupTarget(aGateway({ functionalityType: 'regular', version: '1.2.0' }), HOST);

    expect(target.releaseTag).toBe('gateway/v1.2.0');
    expect(target.distribution).toBe('wso2apip-api-gateway-1.2.0');
  });

  it('downloads the current release when the version is a product line, not a release', () => {
    // "1.0" is what the create form stores, and no track has ever tagged it.
    for (const version of ['1.0', undefined, '']) {
      const target = setupTarget(aGateway({ functionalityType: 'regular', version }), HOST);

      expect(target.version).toBe('2026.09.24');
      expect(downloadCommand(target)).not.toContain('undefined');
    }
  });

  it('builds a download URL that fails loudly on a missing release', () => {
    const target = setupTarget(aGateway({ functionalityType: 'regular', version: '1.0' }), HOST);

    expect(downloadCommand(target)).toBe(
      'mkdir -p ~/wso2-gateways && cd ~/wso2-gateways && \\\n' +
        'curl -fsSLO https://github.com/wso2/api-platform/releases/download/gateway/v2026.09.24/wso2apip-api-gateway-2026.09.24.zip && \\\n' +
        'unzip -oq wso2apip-api-gateway-2026.09.24.zip',
    );
  });

  it('runs the distribution setup before configuring an API or AI gateway', () => {
    const target = setupTarget(aGateway({ version: '1.0' }), HOST);

    expect(prepareCommand(target)).toBe('cd wso2apip-api-gateway-2026.09.24 && ./scripts/setup.sh');
  });

  it('appends the control plane host and token to the env file the gateway reads', () => {
    const target = setupTarget(aGateway({ version: '1.0' }), HOST);

    expect(configureCommand(target, 'secret-token')).toBe(
      "cat >> api-platform.env << 'ENVFILE'\n" +
        `APIP_GW_CONTROLLER_CONTROLPLANE_HOST=${HOST}\n` +
        'APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN=secret-token\n' +
        'ENVFILE',
    );
    expect(startCommand(target)).toBe('docker compose up');
  });

  it('keeps compose variables for the event gateway, which has no setup script', () => {
    const target = setupTarget(aGateway({ functionalityType: 'event', version: '0.9.0' }), HOST);

    expect(prepareCommand(target)).toBe('cd wso2apip-event-gateway-0.9.0');
    expect(configureCommand(target, 'secret-token')).toContain('cat > configs/keys.env');
    expect(configureCommand(target, 'secret-token')).toContain(
      'GATEWAY_REGISTRATION_TOKEN=secret-token',
    );
    expect(startCommand(target)).toBe('docker compose --env-file configs/keys.env up');
  });

  it('points a Docker gateway at host.docker.internal when the control plane is local', () => {
    expect(dockerControlPlaneHost('localhost:9243')).toBe('host.docker.internal:9243');
    expect(dockerControlPlaneHost('127.0.0.1:9243')).toBe('host.docker.internal:9243');
    expect(dockerControlPlaneHost('localhost')).toBe('host.docker.internal');
    expect(dockerControlPlaneHost(HOST)).toBe(HOST);
    expect(dockerControlPlaneHost('localhost.example.com')).toBe('localhost.example.com');

    const target = setupTarget(aGateway({ version: '1.0' }), 'localhost:9243');
    expect(configureCommand(target, 't')).toContain(
      'APIP_GW_CONTROLLER_CONTROLPLANE_HOST=host.docker.internal:9243',
    );
  });

  it('installs the Helm chart under the gateway handle, with the token bound', () => {
    const target = setupTarget(aGateway({ version: '1.2.0' }), HOST);
    const command = helmInstallCommand(target, 'edge-gateway', 'secret-token');

    expect(command).toContain('helm install edge-gateway ');
    expect(command).toContain('--version 1.2.0');
    expect(command).toContain(`controlPlane.host="${HOST}"`);
    expect(command).toContain('token.value="secret-token"');
  });
});
