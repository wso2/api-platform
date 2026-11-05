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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { policyHubHandlers, stubHubFetch } from '@/test/policyHub';
import { server } from '@/test/server';
import { fireEvent, renderWithProviders, screen } from '@/test/utils';
import { AttachedPolicyList } from './AttachedPolicyList';

beforeEach(() => {
  stubHubFetch();
  // An empty hub: every name stays its handle, with no unhandled-request noise.
  server.use(...policyHubHandlers([]));
});

afterEach(() => {
  vi.restoreAllMocks();
});

/** The draggable element holding a policy's name. */
const rowOf = (name: string) => screen.getByText(name).closest('[draggable="true"]') as HTMLElement;

const dataTransfer = () => ({ dropEffect: 'none', effectAllowed: 'none', setData: vi.fn() });

const baseProps = () => ({
  canAdd: true,
  onAdd: vi.fn(),
  onEdit: vi.fn(),
  onRemove: vi.fn(),
  onReorder: vi.fn(),
});

describe('AttachedPolicyList', () => {
  it('does not duplicate the version prefix after a saved policy is reloaded', () => {
    renderWithProviders(
      <AttachedPolicyList
        canAdd
        onAdd={vi.fn()}
        onEdit={vi.fn()}
        onRemove={vi.fn()}
        onReorder={vi.fn()}
        policies={[
          { name: 'stored-policy', version: 'v1' },
          { name: 'catalog-policy', version: '2.0' },
        ]}
      />,
    );

    expect(screen.getByText('v1')).toBeInTheDocument();
    expect(screen.queryByText('vv1')).not.toBeInTheDocument();
    expect(screen.getByText('v2.0')).toBeInTheDocument();
  });

  it('lists policies without add, edit, remove or reorder when read-only', () => {
    const { container } = renderWithProviders(
      <AttachedPolicyList
        canAdd
        onAdd={vi.fn()}
        onEdit={vi.fn()}
        onRemove={vi.fn()}
        onReorder={vi.fn()}
        policies={[{ name: 'cors', version: 'v1' }]}
        readOnly
      />,
    );

    expect(screen.getByText('cors')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Add Policy' })).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Edit policy')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Remove policy')).not.toBeInTheDocument();
    expect(container.querySelector('[draggable="true"]')).toBeNull();
  });

  it('keeps every control when editable', () => {
    const { container } = renderWithProviders(
      <AttachedPolicyList
        canAdd
        onAdd={vi.fn()}
        onEdit={vi.fn()}
        onRemove={vi.fn()}
        onReorder={vi.fn()}
        policies={[{ name: 'cors', version: 'v1' }]}
      />,
    );

    expect(screen.getByRole('button', { name: 'Add Policy' })).toBeInTheDocument();
    expect(screen.getByLabelText('Edit policy')).toBeInTheDocument();
    expect(screen.getByLabelText('Remove policy')).toBeInTheDocument();
    expect(container.querySelector('[draggable="true"]')).not.toBeNull();
  });

  it('shows inherited policies first, read-only, and numbers the resource\u2019s own after them', () => {
    renderWithProviders(
      <AttachedPolicyList
        canAdd
        inheritedPolicies={[
          { name: 'global-a', version: 'v1' },
          { name: 'global-b', version: 'v1' },
        ]}
        onAdd={vi.fn()}
        onEdit={vi.fn()}
        onRemove={vi.fn()}
        onReorder={vi.fn()}
        policies={[{ name: 'own-policy', version: 'v1' }]}
        variant="resource"
      />,
    );

    expect(screen.getByText('global-a')).toBeInTheDocument();
    expect(screen.getByText('global-b')).toBeInTheDocument();
    expect(screen.getByText('own-policy')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('Inherited from global')).toBeInTheDocument();
    // Only the resource's own policy can be edited or removed.
    expect(screen.getAllByLabelText('Edit policy')).toHaveLength(1);
    expect(screen.getAllByLabelText('Remove policy')).toHaveLength(1);
  });

  it('offers the drop hint under the inherited policies when the resource has none of its own', () => {
    renderWithProviders(
      <AttachedPolicyList
        canAdd
        emptyText="Drop here"
        inheritedPolicies={[{ name: 'global-a', version: 'v1' }]}
        onAdd={vi.fn()}
        onEdit={vi.fn()}
        onRemove={vi.fn()}
        onReorder={vi.fn()}
        policies={[]}
        variant="resource"
      />,
    );

    expect(screen.getByText('Drop here')).toBeInTheDocument();
    expect(screen.queryByLabelText('Edit policy')).not.toBeInTheDocument();
  });

  it.each(['list', 'chips', 'resource'] as const)(
    'reorders by dragging one %s item onto another',
    (variant) => {
      const props = baseProps();
      renderWithProviders(
        <AttachedPolicyList
          {...props}
          policies={[
            { name: 'first', version: 'v1' },
            { name: 'second', version: 'v1' },
          ]}
          variant={variant}
        />,
      );

      const transfer = dataTransfer();
      fireEvent.dragStart(rowOf('first'), { dataTransfer: transfer });
      expect(transfer.setData).toHaveBeenCalledWith('application/x-policy-reorder', '0');
      fireEvent.dragOver(rowOf('second'), { dataTransfer: transfer });
      fireEvent.drop(rowOf('second'), { dataTransfer: transfer });

      expect(props.onReorder).toHaveBeenCalledWith(0, 1);
    },
  );

  it('does not reorder when an item is dropped on itself or nothing is being dragged', () => {
    const props = baseProps();
    renderWithProviders(
      <AttachedPolicyList
        {...props}
        policies={[
          { name: 'first', version: 'v1' },
          { name: 'second', version: 'v1' },
        ]}
        variant="chips"
      />,
    );

    // Nothing dragged yet: a drop elsewhere is not a reorder.
    fireEvent.dragOver(rowOf('second'), { dataTransfer: dataTransfer() });
    fireEvent.drop(rowOf('second'), { dataTransfer: dataTransfer() });
    // Dragged onto itself.
    fireEvent.dragStart(rowOf('first'), { dataTransfer: dataTransfer() });
    fireEvent.drop(rowOf('first'), { dataTransfer: dataTransfer() });
    fireEvent.dragEnd(rowOf('first'));

    expect(props.onReorder).not.toHaveBeenCalled();
  });

  it('edits and removes from the list layout and offers Add Policy with a header', async () => {
    const props = baseProps();
    const { user } = renderWithProviders(
      <AttachedPolicyList {...props} policies={[{ name: 'cors', version: 'v1' }]} showHeader />,
    );

    await user.click(screen.getByLabelText('Edit policy'));
    await user.click(screen.getByLabelText('Remove policy'));
    await user.click(screen.getByRole('button', { name: 'Add Policy' }));

    expect(props.onEdit).toHaveBeenCalledWith(0);
    expect(props.onRemove).toHaveBeenCalledWith(0);
    expect(props.onAdd).toHaveBeenCalledTimes(1);
  });

  it('shows the default or a custom empty note in each layout', () => {
    const { unmount } = renderWithProviders(<AttachedPolicyList {...baseProps()} policies={[]} />);
    expect(screen.getByText('No policies attached.')).toBeInTheDocument();
    unmount();

    renderWithProviders(
      <AttachedPolicyList {...baseProps()} emptyText="Drop here" policies={[]} variant="chips" />,
    );
    expect(screen.getByText('Drop here')).toBeInTheDocument();
  });

  it('keeps the hint with its policies when the resource already has some', () => {
    renderWithProviders(
      <AttachedPolicyList
        {...baseProps()}
        emptyText="Drop here"
        policies={[{ name: 'cors', version: 'v1' }]}
      />,
    );

    expect(screen.getByText('Drop here')).toBeInTheDocument();
  });

  it('leaves out the reorder hint on a read-only resource, keeping the inherited legend', () => {
    renderWithProviders(
      <AttachedPolicyList
        {...baseProps()}
        inheritedPolicies={[{ name: 'global-a', version: 'v1' }]}
        policies={[{ name: 'own-policy', version: 'v1' }]}
        readOnly
        variant="resource"
      />,
    );

    expect(screen.getByText('Inherited from global')).toBeInTheDocument();
    expect(screen.queryByText(/Drag to reorder/)).not.toBeInTheDocument();
    expect(screen.queryByText('·')).not.toBeInTheDocument();
  });

  it('shows no caption at all on a read-only resource with nothing inherited', () => {
    renderWithProviders(
      <AttachedPolicyList
        {...baseProps()}
        policies={[{ name: 'own-policy', version: 'v1' }]}
        readOnly
        variant="resource"
      />,
    );

    expect(screen.getByText('own-policy')).toBeInTheDocument();
    expect(screen.queryByText(/Drag to reorder/)).not.toBeInTheDocument();
    expect(screen.queryByText('Inherited from global')).not.toBeInTheDocument();
  });
});
