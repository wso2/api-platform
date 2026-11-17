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

import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import GatewayOverview from './GatewayOverview';
import type { Gateway } from '../types';

/*
 * This page replaces the console's built-in gateway page in the cloud, so it
 * reports the same console features through the host's `trackFeature`.
 */

const gateway: Gateway = {
  id: 'edge',
  name: 'Edge',
  type: 'regular',
  environmentId: 'dev',
  hosting: 'self-hosted',
  url: '',
  status: 'inactive',
  isCritical: false,
  isDefault: false,
  version: '1.2.0',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

const renderOverview = (
  token: string | null = 'registration-token',
  renderPolicies?: (gatewayId: string) => ReactNode
) => {
  const trackFeature = vi.fn();
  render(
    <GatewayOverview
      gateway={gateway}
      environmentNames={new Map([['dev', 'Development']])}
      controlPlaneHost="cp.example.com:443"
      gatewayVersions={[{ type: 'regular', version: '1.2', latestVersion: 'v1.2.0', channel: 'LTS' }]}
      initialRegistrationToken={token}
      onBack={() => {}}
      onEditClick={() => {}}
      onHasActiveRegistrationToken={async () => true}
      onGenerateRegistrationToken={async () => 'fresh-token'}
      onReconfigureRegistrationToken={async () => 'fresh-token'}
      notify={() => {}}
      trackFeature={trackFeature}
      renderPolicies={renderPolicies}
    />
  );
  return trackFeature;
};

beforeEach(() => {
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: vi.fn(() => Promise.resolve()) },
  });
});

describe('GatewayOverview feature usage', () => {
  it('reports a copied setup command as its step, on the install path shown', async () => {
    const trackFeature = renderOverview();

    fireEvent.click(screen.getByRole('button', { name: 'Copy Download command' }));

    await waitFor(() =>
      expect(trackFeature).toHaveBeenCalledWith('apip.gateway.gateway-setup-command-copy', {
        install_method: 'quick-start',
        setup_step: 'download',
      })
    );
  });

  it('reports the Helm install as the kubernetes install step', async () => {
    const trackFeature = renderOverview();

    fireEvent.click(screen.getByRole('tab', { name: /Kubernetes/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Copy Helm install command' }));

    await waitFor(() =>
      expect(trackFeature).toHaveBeenCalledWith('apip.gateway.gateway-setup-command-copy', {
        install_method: 'kubernetes',
        setup_step: 'install',
      })
    );
  });

  it('reports nothing when the clipboard refuses the write', async () => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: vi.fn(() => Promise.reject(new Error('denied'))) },
    });
    const trackFeature = renderOverview();

    fireEvent.click(screen.getByRole('button', { name: 'Copy Download command' }));

    await Promise.resolve();
    await Promise.resolve();
    expect(trackFeature).not.toHaveBeenCalled();
  });

  it('reports Reconfigure once it is confirmed, not when the dialog opens', async () => {
    const trackFeature = renderOverview(null);

    // Enabled once the page has checked the gateway already has a token to revoke.
    const reconfigure = (await screen.findAllByRole('button', { name: 'Reconfigure' }))[0] as HTMLButtonElement;
    await waitFor(() => expect(reconfigure.disabled).toBe(false));
    fireEvent.click(reconfigure);
    expect(trackFeature).not.toHaveBeenCalled();

    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Reconfigure' }));

    expect(trackFeature).toHaveBeenCalledWith('apip.gateway.gateway-token-regenerate-click');
  });

  it('reports the Policies tab once per visit when it shows the policies', () => {
    const trackFeature = renderOverview('registration-token', () => <div>policies</div>);

    fireEvent.click(screen.getByRole('tab', { name: 'Policies' }));
    fireEvent.click(screen.getByRole('tab', { name: 'Configurations' }));
    fireEvent.click(screen.getByRole('tab', { name: 'Policies' }));

    expect(trackFeature.mock.calls).toEqual([['apip.policies.gateway-policies-view']]);
  });

  it('does not report the placeholder shown without policies', () => {
    const trackFeature = renderOverview();

    fireEvent.click(screen.getByRole('tab', { name: 'Policies' }));

    expect(trackFeature).not.toHaveBeenCalled();
  });
});
