/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import type { CloudHostPort } from './hostPort';
import ManagedPortalsList from './ManagedPortalsList';
import { createMockPortalPort } from './mockPort';
import { PortalFeatureProvider } from './portContext';
import type { ManagedPortal, PortalPort } from './types';

const portal: ManagedPortal = {
  id: 'devs',
  handle: 'devs',
  name: 'Developers',
  url: 'https://devs.portals.example',
  status: 'active',
  loginEnvironment: 'production',
};

const host: CloudHostPort = {
  orgHandle: 'acme',
  navigate: () => {},
  notify: () => {},
  permissions: { canAny: () => true },
};

const isDisabled = (element: HTMLElement) => (element as HTMLButtonElement).disabled;

const renderWith = (can: boolean) =>
  render(
    <PortalFeatureProvider value={{ port: createMockPortalPort([portal]), host }}>
      <ManagedPortalsList
        canCreate={can}
        canUpdate={can}
        canDelete={can}
        onCreate={() => {}}
        onEdit={() => {}}
      />
    </PortalFeatureProvider>
  );

describe('ManagedPortalsList permissions', () => {
  it('disables Add, Edit and Delete for a user holding none of the scopes', async () => {
    renderWith(false);

    expect(isDisabled(await screen.findByRole('button', { name: 'Add Portal' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Edit Developers' }))).toBe(true);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete Developers' }))).toBe(true);
  });

  it('offers Add, Edit and Delete to a user holding them', async () => {
    renderWith(true);

    expect(isDisabled(await screen.findByRole('button', { name: 'Add Portal' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Edit Developers' }))).toBe(false);
    expect(isDisabled(screen.getByRole('button', { name: 'Delete Developers' }))).toBe(false);
  });
});

const renderPortals = (portals: ManagedPortal[]) =>
  render(
    <PortalFeatureProvider value={{ port: createMockPortalPort(portals), host }}>
      <ManagedPortalsList canCreate canUpdate canDelete onCreate={() => {}} onEdit={() => {}} />
    </PortalFeatureProvider>
  );

describe('ManagedPortalsList background changes', () => {
  it('names the change a pending portal is running instead of reading as created', async () => {
    renderPortals([
      { ...portal, status: 'pending', change: { op: 'loginEnvironment', loginEnvironment: 'staging' } },
    ]);

    expect(await screen.findByText('Moving sign-in to staging...')).toBeTruthy();
    expect(screen.queryByText('Creating portal...')).toBeNull();
  });

  it('still reads a pending portal without a change as being created', async () => {
    renderPortals([{ ...portal, status: 'pending' }]);

    expect(await screen.findByText('Creating portal...')).toBeTruthy();
  });

  it('shows a failed change with its message and keeps the portal editable', async () => {
    renderPortals([
      {
        ...portal,
        status: 'failed',
        lastChangeError: {
          op: 'loginEnvironment',
          loginEnvironment: 'staging',
          message: 'Moving the sign-in to "staging" stopped part-way. Change the login environment again to finish it.',
        },
      },
    ]);

    expect(await screen.findByText('Update failed')).toBeTruthy();
    expect(screen.getByText(/stopped part-way/)).toBeTruthy();
    expect(screen.queryByText('Creation failed')).toBeNull();
    expect(isDisabled(screen.getByRole('button', { name: 'Edit Developers' }))).toBe(false);
  });
});

/**
 * A port whose list answers with each of `steps` in turn (the last one
 * repeating) and whose get answers with the portal from the latest list.
 */
function steppingPort(steps: ManagedPortal[][]): PortalPort {
  const base = createMockPortalPort();
  let call = 0;
  let latest: ManagedPortal[] = [];
  return {
    ...base,
    async list() {
      latest = steps[Math.min(call, steps.length - 1)];
      call += 1;
      return latest;
    },
    async get(id: string) {
      const found = latest.find((p) => p.id === id);
      if (!found) throw new Error('not found');
      return found;
    },
  };
}

const renderWithPort = (port: PortalPort, notify: CloudHostPort['notify']) =>
  render(
    <PortalFeatureProvider value={{ port, host: { ...host, notify } }}>
      <ManagedPortalsList canCreate canUpdate canDelete onCreate={() => {}} onEdit={() => {}} />
    </PortalFeatureProvider>
  );

const changingPortal: ManagedPortal = {
  ...portal,
  status: 'pending',
  change: { op: 'selfSignup', selfSignup: true },
};

// The list re-polls a pending portal every 3s, so each settles on the second list.
describe('ManagedPortalsList change outcome', () => {
  it('reports a change that settled as updated', async () => {
    const notify = vi.fn();
    renderWithPort(steppingPort([[changingPortal], [portal]]), notify);

    await waitFor(() => expect(notify).toHaveBeenCalledWith('Portal "Developers" updated', 'success'), {
      timeout: 6000,
    });
  }, 10000);

  it('reports a change that failed with its message', async () => {
    const notify = vi.fn();
    const failed: ManagedPortal = {
      ...portal,
      lastChangeError: { op: 'selfSignup', selfSignup: true, message: 'Turning sign-up on failed.' },
    };
    renderWithPort(steppingPort([[changingPortal], [failed]]), notify);

    await waitFor(
      () => expect(notify).toHaveBeenCalledWith('Portal "Developers": Turning sign-up on failed.', 'error'),
      { timeout: 6000 }
    );
    expect(notify).not.toHaveBeenCalledWith('Portal "Developers" updated', 'success');
  }, 10000);

  it('does not report a portal deleted during its change as updated', async () => {
    const notify = vi.fn();
    renderWithPort(steppingPort([[changingPortal], [{ ...portal, status: 'deleting' }]]), notify);

    expect(await screen.findByText('Deleting portal...', undefined, { timeout: 6000 })).toBeTruthy();
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(notify).not.toHaveBeenCalled();
  }, 10000);
});
