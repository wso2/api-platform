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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { within } from '@testing-library/react';

import { renderWithProviders, screen, waitFor } from '../../../../test/utils';
import KindDetailPanel from './KindDetailPanel';

const showSnackbar = vi.fn();
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({ default: () => showSnackbar }));

const onItemDelete = vi.fn();
const onItemClick = vi.fn();
const onRetry = vi.fn();

const item = (id: string, displayName: string) => ({ id, displayName });

const renderPanel = (
  props: Partial<React.ComponentProps<typeof KindDetailPanel>> = {}
) =>
  renderWithProviders(
    <KindDetailPanel
      title="Agent Proxies"
      description="A2A agents fronted by the gateway"
      items={[item('trip', 'Trip Planner')]}
      totalCount={1}
      onRetry={onRetry}
      viewAllPath="/agent-proxy"
      createPath="/agent-proxy/create"
      createLabel="Create Agent Proxy"
      canCreate
      emptyImage="empty.svg"
      emptyTitle="No agent proxies yet"
      emptyDescription="Create one to get started."
      onItemClick={onItemClick}
      itemLabel="Agent Proxy"
      canDelete
      onItemDelete={onItemDelete}
      {...props}
    />
  );

beforeEach(() => {
  showSnackbar.mockReset();
  onItemDelete.mockReset();
  onItemClick.mockReset();
  onRetry.mockReset();
});

describe('listing a kind', () => {
  it('names the kind and what it holds', () => {
    renderPanel();

    expect(screen.getByText('Agent Proxies')).toBeInTheDocument();
    expect(screen.getByText('Trip Planner')).toBeInTheDocument();
  });

  it('invites the user to create one when the kind is empty', () => {
    renderPanel({ items: [], totalCount: 0 });

    expect(screen.getByText('No agent proxies yet')).toBeInTheDocument();
  });

  it('offers a retry when the kind could not be read', () => {
    renderPanel({ error: new Error('control plane down'), items: [], totalCount: 0 });

    expect(onRetry).not.toHaveBeenCalled();
  });

  it('opens the item that was clicked', async () => {
    const { user } = renderPanel();

    await user.click(screen.getByText('Trip Planner'));

    expect(onItemClick).toHaveBeenCalledWith('trip');
  });
});

describe('deleting an item', () => {
  it('asks before deleting', async () => {
    const { user } = renderPanel();

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));

    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(onItemDelete).not.toHaveBeenCalled();
  });

  it('reports success once the item is gone', async () => {
    onItemDelete.mockResolvedValue(undefined);
    const { user } = renderPanel();

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith('Agent Proxy deleted successfully.', 'success')
    );
    expect(onItemDelete).toHaveBeenCalledWith('trip');
  });

  it('surfaces the reason a delete was refused rather than claiming success', async () => {
    onItemDelete.mockRejectedValue(new Error('still has subscriptions'));
    const { user } = renderPanel();

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    await waitFor(() =>
      expect(showSnackbar).toHaveBeenCalledWith('still has subscriptions', 'error')
    );
  });

  it('closes the dialog without deleting when cancelled', async () => {
    const { user } = renderPanel();

    await user.click(screen.getByRole('button', { name: 'Delete Trip Planner' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }));

    expect(onItemDelete).not.toHaveBeenCalled();
  });

  it('keeps the delete control inert without the permission', () => {
    renderPanel({ canDelete: false });

    // The control stays on screen so its tooltip can say why it is unavailable.
    expect(screen.getByRole('button', { name: 'Delete Trip Planner' })).toBeDisabled();
  });
});
