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

import React from 'react';
import { describe, it, expect, vi } from 'vitest';

import { renderWithProviders, screen } from '../../test/utils';
import ProviderRow from './ProviderRow';
import type { TransformerResolution } from '../../utils/transformerResolution';

const cy = (c: HTMLElement, id: string) => c.querySelector(`[data-cyid="${id}"]`) as HTMLElement;
const btn = (c: HTMLElement, id: string) => cy(c, id)?.querySelector('button') ?? cy(c, id);
/** The primary marker is a switch; its input is what a test drives. */
const primaryToggle = (c: HTMLElement) =>
  cy(c, 'provider-row-primary')?.querySelector('input') as HTMLInputElement | null;

const matched: TransformerResolution = { status: 'none', title: 'No transformer needed' };
const needsOne: TransformerResolution = { status: 'unresolved', title: 'No transformer configured' };

const render = (props = {}) =>
  renderWithProviders(
    <ProviderRow
      displayName="Test Anthropic"
      isPrimary={false}
      resolution={matched}
      showPrimaryToggle
      onMakePrimary={vi.fn()}
      onEdit={vi.fn()}
      onRemove={vi.fn()}
      onConfigureTransformer={vi.fn()}
      {...props}
    />
  );

describe('what a provider row shows', () => {
  it('names the provider', () => {
    render();

    expect(screen.getByDisplayValue('Test Anthropic')).toBeInTheDocument();
  });

  it('shows the name as plain text in the plain variant', () => {
    render({ variant: 'plain' });

    expect(screen.getByText('Test Anthropic')).toBeInTheDocument();
  });

  it('shows the request handle, which is what a client sends to select this provider', () => {
    render({ requestHandle: 'test-anthropic' });

    expect(screen.getByText(/Provider ID: test-anthropic/)).toBeInTheDocument();
  });

  it('stays quiet about a provider that needs no transformer', () => {
    const { container } = render({ resolution: matched });

    // Nothing here is waiting on anyone, and a line on every row saying so is
    // a list nobody reads.
    expect(container.textContent).not.toContain('No transformer needed');
  });

  it('reports a provider that cannot serve until someone configures it', () => {
    const { container } = render({ resolution: needsOne });

    // Rendered as "Transformer: <title>", so the title is split across nodes.
    expect(container.textContent).toContain('Transformer: No transformer configured');
  });
});

describe('the primary marker', () => {
  it('is offered when the surface lets the primary move', () => {
    const { container } = render({ showPrimaryToggle: true });

    expect(primaryToggle(container)).toBeInTheDocument();
  });

  it('is withheld where the primary sits in a slot of its own', () => {
    const { container } = render({ showPrimaryToggle: false });

    expect(cy(container, 'provider-row-primary')).toBeNull();
  });

  it('reports which provider is currently primary', () => {
    const { container } = render({ isPrimary: true });

    expect(primaryToggle(container)).toBeChecked();
  });

  it('asks the caller to move the marker when used', async () => {
    const onMakePrimary = vi.fn();
    const { user, container } = render({ onMakePrimary });

    await user.click(primaryToggle(container) as HTMLElement);

    expect(onMakePrimary).toHaveBeenCalled();
  });
});

describe('the row actions', () => {
  it('opens this provider for editing', async () => {
    const onEdit = vi.fn();
    const { user, container } = render({ onEdit });

    await user.click(btn(container, 'provider-row-edit'));

    expect(onEdit).toHaveBeenCalled();
  });

  it('detaches this provider', async () => {
    const onRemove = vi.fn();
    const { user, container } = render({ onRemove });

    await user.click(btn(container, 'provider-row-remove'));

    expect(onRemove).toHaveBeenCalled();
  });

  it('withholds removal when the caller offers no handler', () => {
    const { container } = render({ onRemove: undefined });

    // The last remaining provider cannot be removed: a proxy always has one.
    expect(btn(container, 'provider-row-remove')).toBeDisabled();
  });

  it('offers nothing while disabled', () => {
    const { container } = render({ disabled: true });

    expect(btn(container, 'provider-row-edit')).toBeDisabled();
    expect(btn(container, 'provider-row-remove')).toBeDisabled();
  });
});
