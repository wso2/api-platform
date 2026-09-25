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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, within } from '@testing-library/react';

import { notifyForbidden, resetForbiddenNotice } from '../api/core/sessionEvents';
import { resetPermissionWarnings } from './evaluate';

import { makeAuthState } from '../test/mockAuthState';
import { renderWithProviders } from '../test/utils';
import { PermissionProvider } from './PermissionProvider';
import { usePermissions } from './PermissionContext';
import { useActionPermission, useCan, useCanAny, useHasScope } from './useCan';
import type { PermissionMode } from './evaluate';

/**
 * Reports every answer as text, so a test asserts on what a component would
 * actually see rather than on the shape of the context value.
 */
function Probe() {
  const { isLoading, mode } = usePermissions();
  const canCreate = useCan('CreateProject');
  const canDelete = useCan('DeleteProject');
  const canDeployOrUndeploy = useCanAny(['DeployAPI', 'UndeployDeployment']);
  const isKeyAdmin = useHasScope('ap:api_key:all:manage');
  const del = useActionPermission('DeleteProject');

  return (
    <ul>
      <li>mode:{mode}</li>
      <li>loading:{String(isLoading)}</li>
      <li>create:{String(canCreate)}</li>
      <li>delete:{String(canDelete)}</li>
      <li>deployAny:{String(canDeployOrUndeploy)}</li>
      <li>keyAdmin:{String(isKeyAdmin)}</li>
      <li>reason:{del.reason}</li>
      <li>tooltip:{del.tooltip ?? 'none'}</li>
    </ul>
  );
}

/**
 * Renders the probe for one scope set and returns a reader for its answers.
 *
 * The reader is bound to *this* render's container rather than the whole
 * screen, because several tests below render twice to compare two scope sets
 * and RTL leaves both trees mounted.
 */
const renderWithScopes = (
  scopes: string[] | undefined,
  { mode = 'enforce', status }: { mode?: PermissionMode; status?: 'loading' } = {},
) => {
  const { container } = renderWithProviders(
    <PermissionProvider mode={mode}>
      <Probe />
    </PermissionProvider>,
    {
      authState: makeAuthState({
        ...(status === 'loading' ? { status: 'loading' } : {}),
        user: {
          name: 'Test User',
          email: 'test.user@example.com',
          ...(scopes ? { scopes } : {}),
        },
      }),
    },
  );

  return (label: string) =>
    within(container).getByText((_, element) =>
      Boolean(element?.tagName === 'LI' && element.textContent?.startsWith(`${label}:`)),
    ).textContent;
};

describe('PermissionProvider', () => {
  it('derives permissions from the session scope claim', () => {
    const shown = renderWithScopes(['ap:project:create', 'ap:project:read']);

    expect(shown('create')).toBe('create:true');
    expect(shown('delete')).toBe('delete:false');
    expect(shown('reason')).toBe('reason:missing-scope');
  });

  it('accepts the broader manage scope, because the spec lists it', () => {
    const shown = renderWithScopes(['ap:project:manage']);

    expect(shown('create')).toBe('create:true');
    expect(shown('delete')).toBe('delete:true');
  });

  it('offers an action once any one of several operations is reachable', () => {
    const readOnly = renderWithScopes(['ap:rest_api:read']);
    expect(readOnly('deployAny')).toBe('deployAny:false');

    const deployer = renderWithScopes(['ap:rest_api:deployment:manage']);
    expect(deployer('deployAny')).toBe('deployAny:true');
  });

  it('supplies translated copy for a denied action and none for an allowed one', () => {
    const denied = renderWithScopes([]);
    expect(denied('tooltip')).toContain("don't have permission");

    const allowed = renderWithScopes(['ap:project:manage']);
    expect(allowed('tooltip')).toBe('tooltip:none');
  });

  it('never denies while the session is still resolving', () => {
    const shown = renderWithScopes(undefined, { status: 'loading' });

    expect(shown('loading')).toBe('loading:true');
    expect(shown('delete')).toBe('delete:true');
    expect(shown('reason')).toBe('reason:loading');
  });

  it('treats an absent scope claim per mode, not as an empty grant', () => {
    // Role mode / scope_validation=false: the token carries no scope claim.
    const permissive = renderWithScopes(undefined, { mode: 'permissive' });
    expect(permissive('delete')).toBe('delete:true');
    expect(permissive('reason')).toBe('reason:unknown-scopes');

    const enforcing = renderWithScopes(undefined, { mode: 'enforce' });
    expect(enforcing('delete')).toBe('delete:false');
    expect(enforcing('reason')).toBe('reason:unknown-scopes');

    // An empty array is a real answer and is not the same thing.
    const nothingGranted = renderWithScopes([], { mode: 'permissive' });
    expect(nothingGranted('reason')).toBe('reason:missing-scope');
  });

  it('resolves an override scope that maps to no single operation', () => {
    const admin = renderWithScopes(['ap:api_key:all:manage']);
    expect(admin('keyAdmin')).toBe('keyAdmin:true');

    const reader = renderWithScopes(['ap:api_key:read']);
    expect(reader('keyAdmin')).toBe('keyAdmin:false');
  });

  it('defaults to the flag-derived mode, which is permissive until enabled', () => {
    const { container } = renderWithProviders(
      <PermissionProvider>
        <Probe />
      </PermissionProvider>,
      { authState: makeAuthState() },
    );

    expect(within(container).getByText('mode:permissive')).toBeInTheDocument();
  });

  it('throws when used with no provider above it, rather than failing open', () => {
    // Plain `render`, not `renderWithProviders`: the latter mounts a provider.
    expect(() => render(<Probe />)).toThrow(
      /usePermissions must be used within PermissionProvider/,
    );
  });
});

describe('PermissionProvider drift detection', () => {
  beforeEach(() => {
    resetForbiddenNotice();
    resetPermissionWarnings();
  });
  afterEach(() => vi.restoreAllMocks());

  it('warns when the server refuses an operation it had offered', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    // Holds the scope, so the console would have rendered the control enabled.
    renderWithScopes(['ap:project:manage']);

    notifyForbidden('DeleteProject');

    expect(warn).toHaveBeenCalledTimes(1);
    expect(warn.mock.calls[0][0]).toMatch(/DeleteProject/);
  });

  it('stays silent when the refusal matches what it predicted', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    renderWithScopes(['ap:project:read']);

    notifyForbidden('DeleteProject');

    expect(warn).not.toHaveBeenCalled();
  });

  it('ignores a 403 from a request that carried no operation name', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    renderWithScopes(['ap:project:manage']);

    notifyForbidden(undefined);

    expect(warn).not.toHaveBeenCalled();
  });
});
