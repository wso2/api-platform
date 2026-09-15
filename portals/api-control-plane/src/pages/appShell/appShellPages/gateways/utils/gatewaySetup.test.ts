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
import { configureCommand, downloadCommand, helmInstallCommand, setupTarget } from './gatewaySetup';

const HOST = 'connect.example.com';

/**
 * These strings are pasted into a user's terminal, so the assertions are on the
 * exact text rather than on "contains something plausible".
 */
describe('gatewaySetup', () => {
  it('names the distribution after the gateway functionality and version', () => {
    expect(
      setupTarget(aGateway({ functionalityType: 'ai', version: '1.1.0' }), HOST),
    ).toMatchObject({
      distribution: 'wso2apip-ai-gateway-1.1.0',
      releaseTag: 'ai-gateway/v1.1.0',
      version: '1.1.0',
    });

    expect(
      setupTarget(aGateway({ functionalityType: 'event', version: '2.0' }), HOST),
    ).toMatchObject({ distribution: 'wso2apip-event-gateway-2.0' });
  });

  it('falls back to a known version for a gateway registered without one', () => {
    // `version` is optional in the spec, and a download URL built from
    // `undefined` would 404 rather than fail visibly here.
    const target = setupTarget(aGateway({ version: undefined }), HOST);

    expect(target.version).toBe('1.0');
    expect(downloadCommand(target)).not.toContain('undefined');
  });

  it('builds a download URL from the release tag and archive name', () => {
    const target = setupTarget(aGateway({ functionalityType: 'regular', version: '1.0' }), HOST);

    expect(downloadCommand(target)).toBe(
      'curl -sLO https://github.com/wso2/api-platform/releases/download/api-gateway/v1.0/wso2apip-api-gateway-1.0.zip && \\\n' +
        'unzip wso2apip-api-gateway-1.0.zip',
    );
  });

  it('writes the control plane host and token into the gateway env file', () => {
    const target = setupTarget(aGateway({ version: '1.0' }), HOST);
    const command = configureCommand(target, 'secret-token');

    expect(command).toContain('wso2apip-api-gateway-1.0/configs/keys.env');
    expect(command).toContain(`GATEWAY_CONTROLPLANE_HOST=${HOST}`);
    expect(command).toContain('GATEWAY_REGISTRATION_TOKEN=secret-token');
  });

  it('installs the Helm chart under the gateway handle, with the token bound', () => {
    const target = setupTarget(aGateway({ version: '1.0' }), HOST);
    const command = helmInstallCommand(target, 'edge-gateway', 'secret-token');

    expect(command).toContain('helm install edge-gateway ');
    expect(command).toContain('--version 1.0');
    expect(command).toContain(`controlPlane.host="${HOST}"`);
    expect(command).toContain('token.value="secret-token"');
  });
});
