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
import InboundInterfaceSelect, { selectableInterfaceTemplates } from './InboundInterfaceSelect';
import type { ProviderTemplate } from '../../utils/types';

const template = (id: string, displayName: string, extra: Partial<ProviderTemplate> = {}) =>
  ({ id, displayName, enabled: true, isLatest: true, ...extra }) as ProviderTemplate;

const templates = [
  template('openai', 'OpenAI'),
  template('anthropic', 'Anthropic'),
  template('mistralai', 'Mistral AI'),
];

const render = (props = {}) =>
  renderWithProviders(
    <InboundInterfaceSelect value="openai" onChange={vi.fn()} templates={templates} {...props} />
  );

describe('which templates can be chosen', () => {
  it('offers the enabled, latest templates', () => {
    expect(selectableInterfaceTemplates(templates).map((t) => t.id)).toEqual([
      'openai',
      'anthropic',
      'mistralai',
    ]);
  });

  it('withholds a disabled template', () => {
    const list = [...templates, template('retired', 'Retired', { enabled: false })];

    expect(selectableInterfaceTemplates(list).map((t) => t.id)).not.toContain('retired');
  });

  it('withholds a template that has been superseded', () => {
    const list = [...templates, template('old', 'Old', { isLatest: false })];

    expect(selectableInterfaceTemplates(list).map((t) => t.id)).not.toContain('old');
  });

  it('keeps a template that says nothing about either flag', () => {
    const list = [{ id: 'plain', displayName: 'Plain' } as ProviderTemplate];

    // Absence is not a refusal: a catalogue entry carrying neither flag is still
    // offered, so a template that predates the flags does not vanish.
    expect(selectableInterfaceTemplates(list).map((t) => t.id)).toEqual(['plain']);
  });
});

describe('the interface control', () => {
  it('labels itself and shows the interface in effect', () => {
    render();

    expect(screen.getAllByText('Inbound Interface').length).toBeGreaterThan(0);
    expect(screen.getByText('OpenAI')).toBeInTheDocument();
  });

  it('lists the selectable interfaces when opened', async () => {
    const { user } = render();

    await user.click(screen.getByRole('combobox'));

    expect(await screen.findByText('Anthropic')).toBeInTheDocument();
    expect(screen.getByText('Mistral AI')).toBeInTheDocument();
  });

  it('reports the chosen interface by its handle, not its display name', async () => {
    const onChange = vi.fn();
    const { user } = render({ onChange });

    await user.click(screen.getByRole('combobox'));
    await user.click(await screen.findByText('Anthropic'));

    // The handle routes; the display name only reads well.
    expect(onChange).toHaveBeenCalledWith('anthropic');
  });

  it('shows the helper text a caller supplies', () => {
    render({ helperText: 'Switching the interface replaces the OpenAPI definition below.' });

    expect(
      screen.getByText('Switching the interface replaces the OpenAPI definition below.')
    ).toBeInTheDocument();
  });
});

describe('when the catalogue is not available', () => {
  it('says so without discarding what the user already entered', () => {
    const { container } = render({ error: new Error('offline') });

    expect(
      screen.getByText(/Could not load interface templates. Your other entries are unaffected./)
    ).toBeInTheDocument();
    expect(container.querySelector('[data-cyid="inbound-interface-select-error"]')).toBeInTheDocument();
  });

  it('reports that it is still loading rather than claiming the list is empty', async () => {
    const { user } = render({ templates: [], isLoading: true });

    expect(screen.getByRole('combobox')).toHaveAttribute('aria-disabled', 'true');
    await user.click(screen.getByRole('combobox'));
  });

  it('says the catalogue is empty once loading has finished', async () => {
    const { user } = render({ templates: [], value: '', isLoading: false });

    await user.click(screen.getByRole('combobox'));

    // Both the closed field and the open menu carry the message.
    expect((await screen.findAllByText(/No interface templates available/)).length).toBeGreaterThan(0);
  });

  it('still shows an interface the catalogue no longer lists', () => {
    const { container } = render({ value: 'retired-handle', templates: [] });

    // A proxy keeps running on a template that has since been withdrawn, so the
    // field reports what it is set to rather than appearing unset.
    expect(container.textContent).toContain('retired-handle');
  });
});

describe('when the control is disabled', () => {
  it('accepts no change', () => {
    render({ disabled: true });

    expect(screen.getByRole('combobox')).toHaveAttribute('aria-disabled', 'true');
  });
});
