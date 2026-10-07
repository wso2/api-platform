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

import { renderWithProviders, screen, waitFor } from '../../../../test/utils';
import AgentPolicyMapper from './AgentPolicyMapper';

const onAddPolicy = vi.fn();
const onUpdatePolicy = vi.fn();
const onRemovePolicy = vi.fn();
const onReorderPolicies = vi.fn();

const hubPolicy = (name: string) => ({
  name,
  displayName: name,
  version: '1.0',
  description: `${name} guardrail`,
  category: 'GUARDRAILS',
  policyAttributes: [],
});

vi.mock('../../../../apis/policyHubApis', () => ({
  getPolicies: vi.fn().mockResolvedValue({ data: [] }),
  getGuardrails: vi.fn().mockResolvedValue({ data: [] }),
}));
vi.mock('../../../../apis/gatewayPolicyApis', () => ({
  getGatewayCustomPolicies: vi.fn().mockResolvedValue({ list: [] }),
}));
vi.mock('../../../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

import { getPolicies, getGuardrails } from '../../../../apis/policyHubApis';
import { getGatewayCustomPolicies } from '../../../../apis/gatewayPolicyApis';

const selected = (instanceId: string, displayName = 'guard') => ({
  instanceId,
  policyName: displayName,
  displayName,
  version: '1.0',
  params: {},
});

const renderMapper = (
  props: Partial<React.ComponentProps<typeof AgentPolicyMapper>> = {}
) =>
  renderWithProviders(
    <AgentPolicyMapper
      title="Global Operation Policies"
      description="Applies to all A2A operations."
      readOnly={false}
      selectedPolicies={[]}
      onAddPolicy={onAddPolicy}
      onUpdatePolicy={onUpdatePolicy}
      onRemovePolicy={onRemovePolicy}
      onReorderPolicies={onReorderPolicies}
      {...props}
    />
  );

beforeEach(() => {
  onAddPolicy.mockReset();
  onUpdatePolicy.mockReset();
  onRemovePolicy.mockReset();
  onReorderPolicies.mockReset();
  vi.mocked(getPolicies).mockResolvedValue({ data: [] } as never);
  vi.mocked(getGuardrails).mockResolvedValue({ data: [] } as never);
});

describe('the section itself', () => {
  it('names what the policies apply to', () => {
    renderMapper();

    expect(screen.getByText('Global Operation Policies')).toBeInTheDocument();
    expect(screen.getByText('Applies to all A2A operations.')).toBeInTheDocument();
  });

  it('says so plainly when nothing has been added', () => {
    renderMapper();

    expect(screen.getByText('No policies added yet.')).toBeInTheDocument();
  });

  it('lists the policies already attached', () => {
    renderMapper({ selectedPolicies: [selected('a', 'content-safety')] as never });

    expect(screen.getByText(/content-safety/)).toBeInTheDocument();
    expect(screen.queryByText('No policies added yet.')).not.toBeInTheDocument();
  });
});

describe('the policy drawer', () => {
  it('opens on demand and offers a search', async () => {
    const { user } = renderMapper();

    await user.click(screen.getByRole('button', { name: 'Add Policy' }));

    await waitFor(() =>
      expect(screen.getByPlaceholderText('Search policies')).toBeInTheDocument()
    );
  });

  it('lists what the catalogue returned', async () => {
    const catalogue = { data: [hubPolicy('content-safety'), hubPolicy('prompt-guard')] };
    vi.mocked(getPolicies).mockResolvedValue(catalogue as never);
    vi.mocked(getGuardrails).mockResolvedValue(catalogue as never);
    const { user } = renderMapper();

    await user.click(screen.getByRole('button', { name: 'Add Policy' }));

    await waitFor(() => expect(screen.getByText('content-safety')).toBeInTheDocument());
    expect(screen.getByText('prompt-guard')).toBeInTheDocument();
  });

  it('narrows the list to what was searched for', async () => {
    const catalogue = { data: [hubPolicy('content-safety'), hubPolicy('prompt-guard')] };
    vi.mocked(getPolicies).mockResolvedValue(catalogue as never);
    vi.mocked(getGuardrails).mockResolvedValue(catalogue as never);
    const { user } = renderMapper();

    await user.click(screen.getByRole('button', { name: 'Add Policy' }));
    await waitFor(() => screen.getByText('content-safety'));
    await user.type(screen.getByPlaceholderText('Search policies'), 'prompt');

    await waitFor(() =>
      expect(screen.queryByText('content-safety')).not.toBeInTheDocument()
    );
    expect(screen.getByText('prompt-guard')).toBeInTheDocument();
  });

  it('closes again without adding anything', async () => {
    const { user } = renderMapper();

    await user.click(screen.getByRole('button', { name: 'Add Policy' }));
    await waitFor(() => screen.getByPlaceholderText('Search policies'));
    await user.click(screen.getByRole('button', { name: 'Close policy drawer' }));

    await waitFor(() =>
      expect(screen.queryByPlaceholderText('Search policies')).not.toBeInTheDocument()
    );
    expect(onAddPolicy).not.toHaveBeenCalled();
  });

  it('carries on with an empty list when the catalogue cannot be read', async () => {
    vi.mocked(getPolicies).mockRejectedValue(new Error('hub down'));
    const { user } = renderMapper();

    await user.click(screen.getByRole('button', { name: 'Add Policy' }));

    await waitFor(() =>
      expect(screen.getByPlaceholderText('Search policies')).toBeInTheDocument()
    );
  });
});

describe('a gateway-managed section', () => {
  it('offers no way to add a policy', () => {
    renderMapper({ readOnly: true });

    expect(screen.getByRole('button', { name: 'Add Policy' })).toBeDisabled();
  });
});

describe('an attached policy', () => {
  const attached = [selected('one', 'content-safety'), selected('two', 'prompt-guard')];

  it('can be taken off the list', async () => {
    const { user } = renderMapper({ selectedPolicies: attached as never });

    await user.click(screen.getAllByRole('button', { name: 'Remove guardrail' })[0]);

    expect(onRemovePolicy).toHaveBeenCalledWith('one');
  });

  it('opens its settings when clicked, not the catalogue list', async () => {
    const { user } = renderMapper({ selectedPolicies: attached as never });

    await user.click(screen.getByRole('button', { name: /content-safety/ }));

    // Editing opens the drawer straight on the policy's own settings, so the
    // catalogue search is not part of that view.
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Close policy drawer' })).toBeInTheDocument()
    );
    expect(screen.queryByPlaceholderText('Search policies')).not.toBeInTheDocument();
  });

  // The pill writes to dataTransfer, which jsdom does not supply on its own.
  const dragData = () => ({
    effectAllowed: '',
    dropEffect: '',
    setData: vi.fn(),
    getData: vi.fn(),
  });

  it('is reordered when dragged onto another', () => {
    renderMapper({ selectedPolicies: attached as never });
    const dataTransfer = dragData();
    const first = screen.getByLabelText(/Drag to reorder content-safety/);
    const second = screen.getByLabelText(/Drag to reorder prompt-guard/);

    fireEvent.dragStart(first, { dataTransfer });
    fireEvent.dragOver(second, { dataTransfer });
    fireEvent.drop(second, { dataTransfer });

    expect(onReorderPolicies).toHaveBeenCalledWith('one', 'two');
  });

  it('is left where it was when dropped on itself', () => {
    renderMapper({ selectedPolicies: attached as never });
    const dataTransfer = dragData();
    const first = screen.getByLabelText(/Drag to reorder content-safety/);

    fireEvent.dragStart(first, { dataTransfer });
    fireEvent.drop(first, { dataTransfer });

    expect(onReorderPolicies).not.toHaveBeenCalled();
  });

  it('offers no remove control on a gateway-managed section', () => {
    renderMapper({ selectedPolicies: attached as never, readOnly: true });

    expect(screen.queryByRole('button', { name: 'Remove guardrail' })).not.toBeInTheDocument();
  });
});

describe('picking a policy from the drawer', () => {
  it('opens its settings rather than adding it blind', async () => {
    const catalogue = { data: [hubPolicy('content-safety')] };
    vi.mocked(getPolicies).mockResolvedValue(catalogue as never);
    vi.mocked(getGuardrails).mockResolvedValue(catalogue as never);
    const { user } = renderMapper();

    await user.click(screen.getByRole('button', { name: 'Add Policy' }));
    await waitFor(() => screen.getByText('content-safety'));
    await user.click(screen.getByText('content-safety'));

    expect(onAddPolicy).not.toHaveBeenCalled();
  });
});

describe('custom policies synced from a gateway', () => {
  const customPolicy = {
    uuid: 'cp-1',
    name: 'tenant-guard',
    displayName: 'Tenant Guard',
    version: 'v2.3.1',
    description: 'a policy authored on the gateway',
    provider: 'acme',
    policyDefinition: { name: 'tenant-guard', parameters: [] },
  };

  it('lists them beside the hub guardrails', async () => {
    vi.mocked(getGatewayCustomPolicies).mockResolvedValue({ list: [customPolicy] } as never);
    const { user } = renderMapper();

    await user.click(screen.getByRole('button', { name: 'Add Policy' }));

    await waitFor(() => expect(screen.getByText('Tenant Guard')).toBeInTheDocument());
  });

  it('shows the version as major.minor, matching the hub ones', async () => {
    vi.mocked(getGatewayCustomPolicies).mockResolvedValue({ list: [customPolicy] } as never);
    const { user } = renderMapper();

    await user.click(screen.getByRole('button', { name: 'Add Policy' }));

    // Custom policies arrive as full semver with a "v" prefix; the drawer shows 2.3.
    await waitFor(() => expect(screen.getByText(/2\.3/)).toBeInTheDocument());
  });

  it('opens its inline definition without a hub lookup', async () => {
    vi.mocked(getGatewayCustomPolicies).mockResolvedValue({ list: [customPolicy] } as never);
    const { user } = renderMapper();

    await user.click(screen.getByRole('button', { name: 'Add Policy' }));
    await waitFor(() => screen.getByText('Tenant Guard'));
    await user.click(screen.getByText('Tenant Guard'));

    // The drawer moves to the policy's own settings, built from the definition
    // the gateway supplied inline.
    await waitFor(() =>
      expect(screen.queryByPlaceholderText('Search policies')).not.toBeInTheDocument()
    );
  });

  it('says so when a custom policy carries no definition', async () => {
    vi.mocked(getGatewayCustomPolicies).mockResolvedValue({
      list: [{ ...customPolicy, policyDefinition: undefined }],
    } as never);
    const { user } = renderMapper();

    await user.click(screen.getByRole('button', { name: 'Add Policy' }));
    await waitFor(() => screen.getByText('Tenant Guard'));
    await user.click(screen.getByText('Tenant Guard'));

    await waitFor(() =>
      expect(screen.getByText(/No definition available/)).toBeInTheDocument()
    );
  });

  it('carries on with the hub list when the gateway policies cannot be read', async () => {
    vi.mocked(getGatewayCustomPolicies).mockRejectedValue(new Error('gateway down'));
    const catalogue = { data: [hubPolicy('content-safety')] };
    vi.mocked(getPolicies).mockResolvedValue(catalogue as never);
    vi.mocked(getGuardrails).mockResolvedValue(catalogue as never);
    const { user } = renderMapper();

    await user.click(screen.getByRole('button', { name: 'Add Policy' }));

    await waitFor(() => expect(screen.getByText('content-safety')).toBeInTheDocument());
  });
});
