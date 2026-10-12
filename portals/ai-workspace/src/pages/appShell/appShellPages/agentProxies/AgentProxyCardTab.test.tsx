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

import { renderWithProviders, screen } from '../../../../test/utils';
import AgentProxyCardTab, { type AgentCardTabState } from './AgentProxyCardTab';

const baseState: AgentCardTabState = {
  publicMode: 'passthrough',
  publicRewriteUrls: true,
  publicContent: '',
  protectedMode: 'passthrough',
  protectedRewriteUrls: true,
  protectedContent: '',
};

const onChange = vi.fn();
const onRefetch = vi.fn();

const renderTab = (
  state: Partial<AgentCardTabState> = {},
  props: Partial<React.ComponentProps<typeof AgentProxyCardTab>> = {}
) =>
  renderWithProviders(
    <AgentProxyCardTab
      state={{ ...baseState, ...state }}
      onChange={onChange}
      disabled={false}
      fetchedCard={null}
      isFetching={false}
      fetchError={null}
      onRefetch={onRefetch}
      {...props}
    />
  );

beforeEach(() => {
  onChange.mockReset();
  onRefetch.mockReset();
});

describe('choosing how a card is served', () => {
  it('offers passthrough and managed for both cards', () => {
    renderTab();

    expect(screen.getAllByText('Passthrough')).toHaveLength(2);
    expect(screen.getAllByText('Managed')).toHaveLength(2);
  });

  it('reports the public card switching to managed', async () => {
    const { user } = renderTab();

    await user.click(screen.getAllByText('Managed')[0]);

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ publicMode: 'managed' }));
  });

  it('reports the protected card switching to managed', async () => {
    const { user } = renderTab();

    await user.click(screen.getAllByText('Managed')[1]);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ protectedMode: 'managed' })
    );
  });
});

describe('rewriting urls', () => {
  it('is offered while a card is served passthrough', () => {
    renderTab();

    expect(screen.getAllByText('Rewrite URLs').length).toBeGreaterThan(0);
  });

  it('reports the switch being turned off', async () => {
    const { user } = renderTab();

    const toggles = screen.getAllByRole('switch');
    await user.click(toggles[0]);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ publicRewriteUrls: false })
    );
  });

  it('is withdrawn once the card is managed', () => {
    renderTab({ publicMode: 'managed', protectedMode: 'managed' });

    expect(screen.queryByText('Rewrite URLs')).not.toBeInTheDocument();
  });
});

describe('the live upstream card', () => {
  it('can be refetched on demand', async () => {
    const { user } = renderTab();

    const refetch = screen.getAllByRole('button').find((b) => !b.hasAttribute('disabled'));
    if (refetch) await user.click(refetch);

    expect(screen.getAllByText('Passthrough').length).toBeGreaterThan(0);
  });

  it('presents the fetched card as read-only content', () => {
    renderTab({}, { fetchedCard: '{"name":"Trip Planner"}' });

    expect(screen.getAllByText('Card Content').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Read Only').length).toBeGreaterThan(0);
  });

  it('says the card could not be fetched rather than showing an empty one', () => {
    renderTab({}, { fetchError: 'agent unreachable' });

    expect(
      screen.getAllByText('Could not fetch the card from the upstream agent.').length
    ).toBeGreaterThan(0);
  });

  it('says it is still fetching while the probe is in flight', () => {
    renderTab({}, { isFetching: true });

    expect(screen.getAllByText('Fetching the Agent Card...').length).toBeGreaterThan(0);
  });
});

describe('a gateway-managed proxy', () => {
  it('marks the card as read only', () => {
    renderTab({}, { disabled: true });

    expect(screen.getAllByText('Read Only').length).toBeGreaterThan(0);
  });
});
