/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import type { CloudHostPort } from './hostPort';
import ManagedPortalsList from './ManagedPortalsList';
import { createMockPortalPort } from './mockPort';
import { PortalFeatureProvider } from './portContext';
import type { ManagedPortal } from './types';

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
