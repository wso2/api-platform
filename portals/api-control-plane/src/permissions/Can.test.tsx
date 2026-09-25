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

import { describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';
import { Button } from '@wso2/oxygen-ui';

import { ForbiddenState } from '../components/StateViews';
import { makeAuthState } from '../test/mockAuthState';
import { renderWithProviders } from '../test/utils';
import { Can } from './Can';
import { PermissionProvider } from './PermissionProvider';

/** Renders `ui` under a provider holding exactly `scopes`, enforcing them. */
const renderGated = (ui: React.ReactElement, scopes: string[]) =>
  renderWithProviders(<PermissionProvider mode="enforce">{ui}</PermissionProvider>, {
    authState: makeAuthState({
      user: {
        name: 'Test User',
        email: 'test.user@example.com',
        scopes,
      },
    }),
  });

describe('Can', () => {
  it('renders children when the operation is allowed', () => {
    renderGated(
      <Can do="CreateProject">
        <Button>New project</Button>
      </Can>,
      ['ap:project:create'],
    );

    expect(screen.getByRole('button', { name: 'New project' })).toBeEnabled();
  });

  it('renders nothing when denied and no fallback is given', () => {
    renderGated(
      <Can do="CreateProject">
        <Button>New project</Button>
      </Can>,
      ['ap:project:read'],
    );

    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('substitutes the fallback when denied — a page body becomes ForbiddenState', () => {
    renderGated(
      <Can do="ListProjects" fallback={<ForbiddenState />}>
        <div>Project list</div>
      </Can>,
      [],
    );

    expect(screen.queryByText('Project list')).not.toBeInTheDocument();
    expect(screen.getByText("You don't have permission to view this")).toBeInTheDocument();
  });

  it('disables the control instead of removing it when asked to', () => {
    renderGated(
      <Can do="DeleteProject" denied="disable">
        <Button onClick={vi.fn()}>Delete</Button>
      </Can>,
      ['ap:project:read'],
    );

    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled();
  });

  it('explains a disabled control on hover, through the interactive span wrapper', async () => {
    const { user } = renderGated(
      <Can do="DeleteProject" denied="disable">
        <Button>Delete</Button>
      </Can>,
      ['ap:project:read'],
    );

    // Hovering the button itself cannot work: MUI sets pointer-events: none on
    // a disabled control. The span is what makes the tooltip reachable at all,
    // so this asserts the fix rather than the copy.
    const button = screen.getByRole('button', { name: 'Delete' });
    await user.hover(button.parentElement as HTMLElement);

    expect(await screen.findByRole('tooltip')).toHaveTextContent(/don't have permission/);
  });

  it('leaves an allowed control alone, including its own disabled state', () => {
    renderGated(
      <Can do="DeleteProject" denied="disable">
        <Button disabled>Delete</Button>
      </Can>,
      ['ap:project:manage'],
    );

    // Allowed, so `Can` renders the child untouched — a control disabled for
    // its own reasons (a pending mutation, an invalid form) stays disabled.
    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled();
  });

  it('treats several operations as any-of, for a menu trigger', () => {
    renderGated(
      <Can do={['DeployAPI', 'UndeployDeployment']}>
        <Button>Actions</Button>
      </Can>,
      ['ap:rest_api:deployment:manage'],
    );
    expect(screen.getByRole('button', { name: 'Actions' })).toBeInTheDocument();
  });

  it('hides a menu trigger when none of its operations is reachable', () => {
    renderGated(
      <Can do={['DeployAPI', 'UndeployDeployment']}>
        <Button>Actions</Button>
      </Can>,
      ['ap:rest_api:read'],
    );
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('fails loudly rather than rendering an unentitled control it cannot disable', () => {
    expect(() =>
      renderGated(
        <Can do="DeleteProject" denied="disable">
          plain text
        </Can>,
        [],
      ),
    ).toThrow(/single element child/);
  });
});

describe('ForbiddenState', () => {
  it('accepts overridden copy for a page that can be more specific', () => {
    renderWithProviders(<ForbiddenState title="Gateways are not available to you" />);

    expect(screen.getByText('Gateways are not available to you')).toBeInTheDocument();
    // The default description still explains who can fix it.
    expect(screen.getByText(/administrator in your organization/)).toBeInTheDocument();
  });
});
