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

import { fireEvent, render, screen, waitFor } from '@testing-library/react';
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

type Can = {
  create: boolean;
  update: boolean;
  delete: boolean;
  readConfiguration: boolean;
  writeConfiguration: boolean;
};

const all = (held: boolean): Can => ({
  create: held,
  update: held,
  delete: held,
  readConfiguration: held,
  writeConfiguration: held,
});

const listWith = (can: Can) => (
    <GatewaysList
      gateways={[gateway]}
      environments={[{ id: 'dev', name: 'dev' }]}
      port={port}
      canCreate={can.create}
      canUpdate={can.update}
      canDelete={can.delete}
      canReadConfiguration={can.readConfiguration}
      canWriteConfiguration={can.writeConfiguration}
      onAddClick={() => {}}
      onEditClick={() => {}}
      onViewClick={() => {}}
      onDelete={() => {}}
    />
);

const renderWith = (can: Can) => render(listWith(can));

describe('GatewaysList permissions', () => {
  it('disables every action for a user holding none of the scopes', () => {
    renderWith(all(false));

    expect(isDisabled(screen.getByRole('button', { name: 'Add Gateway' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Edit Dev edge' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Configure Dev edge' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete Dev edge' }))).toBe(true);
  });

  it('offers every action to a user holding them', () => {
    renderWith(all(true));

    expect(isDisabled(screen.getByRole('button', { name: 'Add Gateway' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Edit Dev edge' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Configure Dev edge' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete Dev edge' }))).toBe(false);
  });

  // Each control answers to its own scope: were Edit gated on create, or
  // Delete on update, the two cases above would still pass.
  it('gates each action on its own scope', () => {
    renderWith({ ...all(false), update: true, readConfiguration: true });

    expect(isDisabled(screen.getByRole('button', { name: 'Add Gateway' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Edit Dev edge' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Configure Dev edge' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete Dev edge' }))).toBe(true);
  });

  // A token refresh can drop the read scope while the drawer is open; left open
  // it would keep polling a configuration the server now refuses.
  it('closes the configuration drawer when configuration-read access is lost', async () => {
    const { rerender } = renderWith(all(true));
    fireEvent.click(screen.getByRole('button', { name: 'Configure Dev edge' }));
    expect(await screen.findByText('Gateway Configuration')).toBeTruthy();

    rerender(listWith({ ...all(true), readConfiguration: false }));

    await waitFor(() => expect(screen.queryByText('Gateway Configuration')).toBeNull());
  });
});
