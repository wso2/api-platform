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
import { describe, expect, it, vi } from 'vitest';

import DeployFeature from './DeployFeature';
import DeployPage from './DeployPage';
import ProviderDeployFeature from './ProviderDeployFeature';
import ProviderDeployPage from './ProviderDeployPage';
import type { CloudHostPort } from './hostPort';
import type { Build, Environment } from './types';

// b1 is what dev runs, so the platform refuses to delete it whatever the user
// holds; b0 sits idle, which is where the scope alone decides.
const builds: Build[] = [
  { buildId: 'b1', createdAt: '2026-01-01T00:00:00Z' },
  { buildId: 'b0', createdAt: '2025-12-31T00:00:00Z' },
];

/** Two environments: dev runs b1 on its gateway, prod has an idle active gateway. */
const environments: Environment[] = [
  {
    name: 'dev',
    gateways: [
      {
        id: 'dev-gw',
        name: 'Dev gateway',
        health: 'active',
        status: 'DEPLOYED',
        isDefault: true,
        deploymentId: 'd1',
        buildId: 'b1',
      },
    ],
  },
  {
    name: 'prod',
    gateways: [
      { id: 'prod-gw', name: 'Prod gateway', health: 'active', status: 'NOT_DEPLOYED', isDefault: true },
    ],
  },
];

const isDisabled = (element: HTMLElement) => (element as HTMLButtonElement).disabled;

// The Stop buttons sit inside collapsed rows, whose text is left out of an
// accessible name, so they are found by their label text — one per gateway, in
// environment order. What is asserted is whether each is disabled, not shown.
const stopButtons = () =>
  screen.getAllByText('Stop deployment').map((label) => label.closest('button') as HTMLButtonElement);

describe('DeployPage permissions', () => {
  const renderWith = (can: { deploy: boolean; undeploy: boolean; deleteBuild: boolean }) =>
    render(
      <DeployPage
        environments={environments}
        builds={builds}
        apiName="orders"
        onAddGateway={() => {}}
        onAddCloudGateway={() => {}}
        takesEndpoint={false}
        ownership="control-plane"
        canDeploy={can.deploy}
        canUndeploy={can.undeploy}
        canDeleteBuild={can.deleteBuild}
        busy={false}
        onDeploy={() => {}}
        onStopGateway={() => {}}
        onDeleteBuild={() => {}}
      />
    );

  it('disables deploy, promote, stop and build deletion for a user holding none of the scopes', () => {
    renderWith({ deploy: false, undeploy: false, deleteBuild: false });

    expect(isDisabled(screen.getByRole('button', { name: 'Deploy' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Promote to prod' }))).toBe(true);
    expect(stopButtons().map(isDisabled)).toEqual([true, true]);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete build b0' }))).toBe(true);
  });

  it('gates each action on its own scope', () => {
    renderWith({ deploy: true, undeploy: false, deleteBuild: true });

    expect(isDisabled(screen.getByRole('button', { name: 'Deploy' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Promote to prod' }))).toBe(false);
    expect(stopButtons().map(isDisabled)).toEqual([true, true]);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete build b0' }))).toBe(false);
    // Held by a gateway: refused for its own reason, scope or no scope.
    expect(isDisabled(screen.getByRole('button', { name: 'Delete build b1' }))).toBe(true);
  });

  it('offers Stop only where there is a deployment to stop, once the scope is held', () => {
    renderWith({ deploy: true, undeploy: true, deleteBuild: true });

    // dev runs a deployment; prod has nothing to stop whatever the user holds.
    expect(stopButtons().map(isDisabled)).toEqual([false, true]);
  });
});

describe('ProviderDeployPage permissions', () => {
  const renderWith = (can: { deploy: boolean; undeploy: boolean; deleteBuild: boolean }) =>
    render(
      <ProviderDeployPage
        environments={[environments[0]]}
        builds={builds}
        upstream={{}}
        canDeploy={can.deploy}
        canStoreCredential
        canUndeploy={can.undeploy}
        canDeleteBuild={can.deleteBuild}
        busy={false}
        onDeploy={() => {}}
        onStopGateway={() => {}}
        onDeleteBuild={() => {}}
      />
    );

  it('disables deploy, stop and build deletion for a user holding none of the scopes', () => {
    renderWith({ deploy: false, undeploy: false, deleteBuild: false });

    expect(isDisabled(screen.getByRole('button', { name: 'Deploy' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Stop' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete build b0' }))).toBe(true);
  });

  it('offers them to a user holding the scopes', () => {
    renderWith({ deploy: true, undeploy: true, deleteBuild: true });

    expect(isDisabled(screen.getByRole('button', { name: 'Deploy' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Stop' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete build b0' }))).toBe(false);
  });
});

/*
 * The credential field is a write of its own — a typed key is stored as a
 * secret before the deploy — so it is gated apart from Deploy: without the
 * secret scope the deploy still goes ahead on each gateway's existing key.
 */
describe('ProviderDeployPage credentials', () => {
  const openDrawerWith = (canStoreCredential: boolean) => {
    render(
      <ProviderDeployPage
        environments={[environments[0]]}
        builds={builds}
        upstream={{ authType: 'api-key' }}
        canDeploy
        canStoreCredential={canStoreCredential}
        canUndeploy
        canDeleteBuild
        busy={false}
        onDeploy={() => {}}
        onStopGateway={() => {}}
        onDeleteBuild={() => {}}
      />
    );
    fireEvent.click(screen.getByRole('button', { name: 'Deploy' }));
  };

  it('disables the key field for a user who may not store a secret, leaving Deploy available', () => {
    openDrawerWith(false);

    expect(isDisabled(screen.getByPlaceholderText('Deploys with the current key'))).toBe(true);
    expect(screen.getAllByRole('button', { name: 'Deploy' }).map(isDisabled)).toContain(false);
  });

  it('offers the key field to a user who may', () => {
    openDrawerWith(true);

    expect(isDisabled(screen.getByPlaceholderText('Leave empty to keep the current key'))).toBe(false);
  });
});

/*
 * The features decide which calls are made from the scopes held, so these run
 * them against a Port whose user holds exactly a given set and assert on the
 * calls, not on what the page then shows.
 */
const portHolding = (held: string[]) => {
  const apiFetch = vi.fn(async () => ({ list: [] }));
  const port: CloudHostPort = {
    orgHandle: 'acme',
    projectHandle: 'shop',
    apiHandle: 'orders',
    navigate: () => {},
    notify: () => {},
    apiFetch: apiFetch as unknown as CloudHostPort['apiFetch'],
    permissions: { canAny: (scopes) => scopes.some((scope) => held.includes(scope)) },
  };
  const paths = () => apiFetch.mock.calls.map((call) => (call as unknown as [string, string])[1]);
  return { port, apiFetch, paths };
};

describe('DeployFeature permissions', () => {
  it('says so and loads nothing when the user may not list deployments', () => {
    const { port, apiFetch } = portHolding(['ap:deployment:deploy']);
    render(<DeployFeature port={port} />);

    expect(screen.getByText(/don't have permission to view deployments/i)).toBeTruthy();
    expect(apiFetch).not.toHaveBeenCalled();
  });

  it('reads the pipeline but not the API for a user holding only deployment scopes', async () => {
    const { port, paths } = portHolding(['ap:deployment:manage']);
    render(<DeployFeature port={port} />);

    await waitFor(() =>
      expect(paths().some((path) => path.includes('/projects/shop/apis/orders/deployments'))).toBe(true)
    );
    expect(paths().some((path) => path.startsWith('/rest-apis/'))).toBe(false);
  });

  it('reads the API too once a scope its read accepts is held', async () => {
    const { port, paths } = portHolding(['ap:deployment:manage', 'ap:rest_api:read']);
    render(<DeployFeature port={port} />);

    await waitFor(() => expect(paths()).toContain('/rest-apis/orders'));
  });
});

describe('ProviderDeployFeature permissions', () => {
  it('says so and loads nothing when the user may not list the deployments', () => {
    const { port, apiFetch } = portHolding(['ap:llm_provider:read']);
    render(<ProviderDeployFeature port={port} artifactHandle="openai" />);

    expect(screen.getByText(/don't have permission to view deployments/i)).toBeTruthy();
    expect(apiFetch).not.toHaveBeenCalled();
  });

  it('still shows the environments when only the build list is refused', async () => {
    const { port, paths } = portHolding(['ap:deployment:read']);
    render(<ProviderDeployFeature port={port} artifactHandle="openai" />);

    expect(await screen.findByText('No environments yet')).toBeTruthy();
    expect(paths()).toContain('/llm-providers/openai/environment-deployments');
    expect(paths()).not.toContain('/llm-providers/openai/builds');
  });

  it('reads the builds once their scope is held', async () => {
    const { port, paths } = portHolding(['ap:deployment:read', 'ap:llm_provider:build:read']);
    render(<ProviderDeployFeature port={port} artifactHandle="openai" />);

    await waitFor(() => expect(paths()).toContain('/llm-providers/openai/builds'));
  });
});
