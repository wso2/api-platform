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

import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import GatewayOverview, { type GatewayOverviewProps } from './GatewayOverview';
import type { Gateway } from '../types';

const gateway: Gateway = {
  id: 'dev-self-hosted',
  name: 'Self-Hosted',
  type: 'regular',
  environmentId: 'dev',
  hosting: 'self-hosted',
  url: 'https://gw.example.com',
  status: 'active',
  isCritical: false,
  isDefault: false,
  version: '1.2',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

const overview = (props: Partial<GatewayOverviewProps> = {}) => (
  <GatewayOverview
    gateway={gateway}
    environmentNames={new Map([['dev', 'Development']])}
    controlPlaneHost="cp.example.com:443"
    gatewayVersions={[]}
    onBack={() => {}}
    onEditClick={() => {}}
    onHasActiveRegistrationToken={async () => true}
    onGenerateRegistrationToken={async () => 'token'}
    onReconfigureRegistrationToken={async () => 'token'}
    notify={() => {}}
    canManageTokens={false}
    {...props}
  />
);

describe('GatewayOverview Policies tab', () => {
  it('shows what the host renders for this gateway', () => {
    const asked: string[] = [];
    render(
      overview({
        renderPolicies: (gatewayId) => {
          asked.push(gatewayId);
          return <p>Policies of {gatewayId}</p>;
        },
      })
    );

    expect(asked).toEqual([]);
    fireEvent.click(screen.getByRole('tab', { name: 'Policies' }));

    expect(screen.getByText('Policies of dev-self-hosted')).toBeTruthy();
    expect(asked).toContain('dev-self-hosted');
  });

  it('says policies are not available when the host supplies no view', () => {
    render(overview());

    fireEvent.click(screen.getByRole('tab', { name: 'Policies' }));

    expect(screen.getByText("Gateway policies aren't available here yet.")).toBeTruthy();
  });
});
