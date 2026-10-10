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

import { fireEvent } from '@testing-library/react';

import { renderWithProviders, screen } from '../../../../test/utils';
import AgentProxyGuardrailsTab from './AgentProxyGuardrailsTab';

// The policy mappers inside the tab offer a catalogue fetched from the gateway;
// an empty catalogue is enough for everything asserted here.
vi.mock('../../../../apis/gatewayPolicyApis', () => ({
  getGatewayCustomPolicies: vi.fn().mockResolvedValue([]),
}));

const policy = (instanceId: string) => ({
  instanceId,
  policyName: 'guard',
  policyVersion: 'v1',
  params: {},
});

const emptyState = {
  globalPolicies: [],
  operationPolicies: {},
  publicCardPolicies: [],
};

const renderTab = (state = emptyState, readOnly = false) => {
  const onChange = vi.fn();
  return {
    onChange,
    ...renderWithProviders(
      <AgentProxyGuardrailsTab state={state as never} onChange={onChange} readOnly={readOnly} />
    ),
  };
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe('the policy sections', () => {
  it('always offers a global and a public card section', () => {
    renderTab();

    expect(screen.getByText('Global Operation Policies')).toBeInTheDocument();
    expect(screen.getByText('Public Agent Card Policies')).toBeInTheDocument();
  });

  it('says so plainly when no operation has policies yet', () => {
    renderTab();

    expect(screen.getByText(/No operation-wise policies yet/)).toBeInTheDocument();
  });
});

describe('which operations are on screen', () => {
  it('shows an operation that already carries policies without it being picked', () => {
    renderTab({ ...emptyState, operationPolicies: { GetTask: [policy('p1')] } });

    expect(screen.getByRole('button', { name: 'Remove GetTask' })).toBeInTheDocument();
  });

  it('shows an operation once it is picked from the dropdown', async () => {
    const { user } = renderTab();

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', { name: /^SendMessage\s/ }));

    expect(screen.getByRole('button', { name: 'Remove SendMessage' })).toBeInTheDocument();
  });

  it('stops offering an operation that is already on screen', async () => {
    const { user } = renderTab({
      ...emptyState,
      operationPolicies: { GetTask: [policy('p1')] },
    });

    await user.click(screen.getByRole('combobox'));

    expect(screen.queryByRole('option', { name: /^GetTask\s/ })).not.toBeInTheDocument();
    expect(screen.getByRole('option', { name: /^SendMessage\s/ })).toBeInTheDocument();
  });

  it('keeps the canonical order regardless of the order operations were added', async () => {
    const { user } = renderTab();

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', { name: /^CancelTask\s/ }));
    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', { name: /^SendMessage\s/ }));

    const shown = screen
      .getAllByRole('button', { name: /^Remove / })
      .map((b) => b.getAttribute('aria-label'));
    expect(shown).toEqual(['Remove SendMessage', 'Remove CancelTask']);
  });
});

describe('removing an operation', () => {
  it('clears the policies it was carrying', async () => {
    const { user, onChange } = renderTab({
      ...emptyState,
      operationPolicies: { GetTask: [policy('p1')] },
    });

    await user.click(screen.getByRole('button', { name: 'Remove GetTask' }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ operationPolicies: {} })
    );
  });

  it('reports no change when the operation carried none', async () => {
    const { user, onChange } = renderTab();

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', { name: /^SendMessage\s/ }));
    await user.click(screen.getByRole('button', { name: 'Remove SendMessage' }));

    expect(onChange).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: 'Remove SendMessage' })).not.toBeInTheDocument();
  });
});

describe('a gateway-managed proxy', () => {
  it('offers no way to add a policy', () => {
    renderTab(emptyState, true);

    screen.getAllByRole('button', { name: 'Add Policy' }).forEach((button) => {
      expect(button).toBeDisabled();
    });
  });
});

describe('changing the policies attached to a section', () => {
  const two = [policy('p1'), policy('p2')];

  it('reports a removal back to the owner', async () => {
    const { user, onChange } = renderTab({
      ...emptyState,
      globalPolicies: two,
    } as never);

    await user.click(screen.getAllByRole('button', { name: 'Remove guardrail' })[0]);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        globalPolicies: [expect.objectContaining({ instanceId: 'p2' })],
      })
    );
  });

  it('reports a reorder back to the owner', () => {
    const { onChange } = renderTab({ ...emptyState, globalPolicies: two } as never);
    const dataTransfer = {
      effectAllowed: '', dropEffect: '', setData: vi.fn(), getData: vi.fn(),
    };
    const pills = screen.getAllByLabelText(/Drag to reorder/);

    fireEvent.dragStart(pills[0], { dataTransfer });
    fireEvent.dragOver(pills[1], { dataTransfer });
    fireEvent.drop(pills[1], { dataTransfer });

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        globalPolicies: [
          expect.objectContaining({ instanceId: 'p2' }),
          expect.objectContaining({ instanceId: 'p1' }),
        ],
      })
    );
  });

  it('keeps the public card policies separate from the global ones', async () => {
    const { user, onChange } = renderTab({
      ...emptyState,
      globalPolicies: [policy('g1')],
      publicCardPolicies: [policy('c1')],
    } as never);

    // The last remove control belongs to the public card section.
    const removes = screen.getAllByRole('button', { name: 'Remove guardrail' });
    await user.click(removes[removes.length - 1]);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        globalPolicies: [expect.objectContaining({ instanceId: 'g1' })],
        publicCardPolicies: [],
      })
    );
  });

  it('reports a removal from one operation without touching another', async () => {
    const { user, onChange } = renderTab({
      ...emptyState,
      operationPolicies: { GetTask: [policy('o1')], CancelTask: [policy('o2')] },
    } as never);

    await user.click(screen.getAllByRole('button', { name: 'Remove guardrail' })[0]);

    const [[next]] = onChange.mock.calls;
    expect(next.operationPolicies.CancelTask).toHaveLength(1);
  });
});
