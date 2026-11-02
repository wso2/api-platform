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

import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import GatewaysList from './GatewaysList';
import type { AIWorkspaceHostPort } from './hostPort';
import type { Gateway } from './types';

const gateway: Gateway = {
  id: 'dev-edge',
  name: 'Dev edge',
  type: 'regular',
  environmentId: 'dev',
  hosting: 'managed',
  url: 'dev-edge.example.com',
  status: 'active',
  isCritical: false,
  isDefault: true,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

const port: AIWorkspaceHostPort = {
  orgHandle: 'acme',
  navigate: () => {},
  notify: () => {},
  apiFetch: async () => undefined,
  gatewayVersions: [],
  permissions: { canAny: () => true },
};

const isDisabled = (element: HTMLElement) => (element as HTMLButtonElement).disabled;

const renderWith = (can: boolean) =>
  render(
    <GatewaysList
      gateways={[gateway]}
      environments={[{ id: 'dev', name: 'dev' }]}
      port={port}
      canCreate={can}
      canUpdate={can}
      canDelete={can}
      canReadConfiguration={can}
      canWriteConfiguration={can}
      onAddClick={() => {}}
      onEditClick={() => {}}
      onViewClick={() => {}}
      onDelete={() => {}}
    />
  );

describe('GatewaysList permissions', () => {
  it('disables every action for a user holding none of the scopes', () => {
    renderWith(false);

    expect(isDisabled(screen.getByRole('button', { name: 'Add Gateway' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Edit Dev edge' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Configure Dev edge' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete Dev edge' }))).toBe(true);
  });

  it('offers every action to a user holding them', () => {
    renderWith(true);

    expect(isDisabled(screen.getByRole('button', { name: 'Add Gateway' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Edit Dev edge' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Configure Dev edge' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete Dev edge' }))).toBe(false);
  });
});
