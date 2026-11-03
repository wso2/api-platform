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
import ProviderTable from './ProviderTable';
import type { ProxyProviderEntry } from '../../utils/types';
import type { TransformerResolution } from '../../utils/transformerResolution';

/** The table keys every control by data-cyid, as the Cypress suite does. */
const cy = (container: HTMLElement, id: string) =>
  container.querySelector(`[data-cyid="${id}"]`) as HTMLElement;
const button = (container: HTMLElement, id: string) =>
  cy(container, id).querySelector('button') ?? cy(container, id);

const entry = (
  id: string,
  isPrimary = false,
  transformer?: ProxyProviderEntry['transformer']
): ProxyProviderEntry => ({ id, isPrimary, transformer });

const resolutions: Record<string, TransformerResolution> = {
  none: { status: 'none', title: 'Not required' },
  auto: {
    status: 'auto',
    title: 'OpenAI to Anthropic Transformer',
    policy: { name: 'openai-to-anthropic-transformer', version: 'v0' } as never,
  },
  unresolved: { status: 'unresolved', title: 'Required' },
};

/** The three states a provider row can be in, in one table. */
const threeProviders = [
  entry('test-openai', true),
  entry('test-anthropic', false, { type: 'openai-to-anthropic-transformer', version: 'v0' } as never),
  entry('test-mistral'),
];

const renderTable = (overrides: Partial<React.ComponentProps<typeof ProviderTable>> = {}) => {
  const props = {
    entries: threeProviders,
    displayNameFor: (e: ProxyProviderEntry) =>
      ({ 'test-openai': 'Test Openai', 'test-anthropic': 'Test Anthropic', 'test-mistral': 'Test Mistral' })[e.id] ??
      e.id,
    handleFor: (e: ProxyProviderEntry) => e.id,
    resolutionFor: (e: ProxyProviderEntry) =>
      e.isPrimary ? resolutions.none : e.transformer ? resolutions.auto : resolutions.unresolved,
    onMakePrimary: vi.fn(),
    onEdit: vi.fn(),
    onRemove: vi.fn(),
    onConfigureTransformer: vi.fn(),
    onRemoveTransformer: vi.fn(),
    ...overrides,
  };
  return { props, ...renderWithProviders(<ProviderTable {...props} />) };
};

describe('the provider table', () => {
  it('lists every attached provider under the four columns', () => {
    renderTable();

    for (const column of ['Provider', 'Transformer policy', 'Primary', 'Actions']) {
      expect(screen.getByText(column)).toBeInTheDocument();
    }
    for (const name of ['Test Openai', 'Test Anthropic', 'Test Mistral']) {
      expect(screen.getByText(name)).toBeInTheDocument();
    }
  });

  it('shows each provider its request handle, which is what a client sends to select it', () => {
    renderTable();

    // The handle, not the display name, is the value that routes.
    expect(screen.getByText(/test-openai/)).toBeInTheDocument();
    expect(screen.getByText(/test-anthropic/)).toBeInTheDocument();
  });

  it('distinguishes a provider that needs no transformer from one that still requires it', () => {
    const { container } = renderTable();

    expect(screen.getByText('Not required')).toBeInTheDocument();
    expect(screen.getByText('Required')).toBeInTheDocument();
    // An attached policy shows as a chip carrying the policy's own name and the
    // major version that will run — not the resolution's display title.
    expect(container.textContent).toContain('openai-to-anthropic-transformer (0)');
  });

  it('counts the providers that cannot yet receive traffic', () => {
    renderTable();

    expect(
      screen.getByText(/1 provider requires a transformer policy before it can receive traffic/)
    ).toBeInTheDocument();
  });

  it('says nothing about missing transformers once every provider has what it needs', () => {
    renderTable({ resolutionFor: () => resolutions.none });

    expect(screen.queryByText(/requires a transformer policy/)).not.toBeInTheDocument();
  });

  it('pluralises the warning when more than one provider is unconfigured', () => {
    renderTable({ resolutionFor: (e: ProxyProviderEntry) => (e.isPrimary ? resolutions.none : resolutions.unresolved) });

    expect(screen.getByText(/2 providers require a transformer policy/)).toBeInTheDocument();
  });
});

describe('acting on a row', () => {
  it('moves the primary marker to the provider whose toggle was used', async () => {
    const { props, user, container } = renderTable();

    await user.click(cy(container, 'provider-row-1-primary').querySelector('input') as HTMLElement);

    expect(props.onMakePrimary).toHaveBeenCalledWith('test-anthropic');
  });

  it('opens the settings for the row that was edited, by its position', async () => {
    const { props, user, container } = renderTable();

    await user.click(button(container, 'provider-row-1-edit'));

    expect(props.onEdit).toHaveBeenCalledWith(1);
  });

  it('detaches the provider that was removed', async () => {
    const { props, user, container } = renderTable();

    await user.click(button(container, 'provider-row-1-remove'));

    expect(props.onRemove).toHaveBeenCalledWith('test-anthropic');
  });

  it('withholds removal from a row the caller says cannot be removed', () => {
    const { container } = renderTable({ canRemove: (e: ProxyProviderEntry) => !e.isPrimary });

    // A proxy always has at least one provider, so the primary cannot be removed.
    expect(button(container, 'provider-row-0-remove')).toBeDisabled();
    expect(button(container, 'provider-row-1-remove')).not.toBeDisabled();
  });

  it('withholds every removal when the caller passes no remove handler', () => {
    const { container } = renderTable({ onRemove: undefined });

    expect(button(container, 'provider-row-0-remove')).toBeDisabled();
    expect(button(container, 'provider-row-2-remove')).toBeDisabled();
  });
});

describe('when the table is disabled', () => {
  it('offers no row actions', () => {
    const { container } = renderTable({ disabled: true });

    for (const id of ['provider-row-0-edit', 'provider-row-0-remove', 'provider-row-1-edit']) {
      expect(button(container, id)).toBeDisabled();
    }
  });
});
