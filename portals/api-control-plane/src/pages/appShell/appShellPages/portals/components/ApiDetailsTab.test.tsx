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

import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';
import type { DraftFormValues } from '../utils/publicationForm';
import { ApiDetailsTab } from './ApiDetailsTab';

const VALUES: DraftFormValues = {
  displayName: 'Loans',
  version: '1.0.0',
  description: 'Manage loans.',
  productionUrl: 'https://api.example.com',
  sandboxUrl: 'https://sandbox.example.com',
  agentVisibility: 'VISIBLE',
};

describe('ApiDetailsTab', () => {
  it('reports each edit with every other value kept, and a blur with the field it left', async () => {
    const onChange = vi.fn();
    const onBlurField = vi.fn();
    const { user } = renderWithProviders(
      <ApiDetailsTab onBlurField={onBlurField} onChange={onChange} values={VALUES} />,
    );

    const name = screen.getByDisplayValue('Loans');
    await user.type(name, 'x');
    await user.tab();

    expect(onChange).toHaveBeenLastCalledWith({ ...VALUES, displayName: 'Loansx' });
    expect(onBlurField).toHaveBeenCalledWith('displayName');
  });

  it('shows every value as read-only when asked to, and never reports an edit', async () => {
    const onChange = vi.fn();
    const { user } = renderWithProviders(<ApiDetailsTab onChange={onChange} readOnly values={VALUES} />);

    for (const value of [VALUES.displayName, VALUES.version, VALUES.description, VALUES.productionUrl, VALUES.sandboxUrl]) {
      expect(screen.getByDisplayValue(value)).toHaveAttribute('readonly');
    }
    expect(screen.getByRole('switch', { name: 'Make this API discoverable by AI agents' })).toBeDisabled();
    await user.type(screen.getByDisplayValue('Loans'), 'x');

    expect(onChange).not.toHaveBeenCalled();
  });

  it('offers a description hint only while the field can be edited', () => {
    const empty = { ...VALUES, description: '' };
    const { rerender } = renderWithProviders(<ApiDetailsTab values={empty} />);
    expect(screen.getByRole('textbox', { name: 'Description' })).toHaveAttribute('placeholder');

    rerender(<ApiDetailsTab readOnly values={empty} />);
    expect(screen.getByRole('textbox', { name: 'Description' })).not.toHaveAttribute('placeholder');
  });

  it('offers the gateway URLs for the Production URL, and reports a pick, a typed URL or a clear', async () => {
    const onChange = vi.fn();
    const options = [{ gatewayName: 'Gateway A', url: 'https://gw-a.example.com/loans' }];
    const { user } = renderWithProviders(
      <ApiDetailsTab onChange={onChange} productionUrlOptions={options} values={VALUES} />,
    );

    const production = screen.getByRole('combobox', { name: 'Production URL' });
    await user.click(production);
    await user.click(await screen.findByRole('option', { name: /gw-a\.example\.com\/loans/ }));
    expect(onChange).toHaveBeenLastCalledWith({ ...VALUES, productionUrl: 'https://gw-a.example.com/loans' });

    await user.clear(production);
    expect(onChange).toHaveBeenLastCalledWith({ ...VALUES, productionUrl: '' });
  });

  it('lets the Sandbox URL be typed and cleared, with nothing to pick from', async () => {
    const onChange = vi.fn();
    const { user } = renderWithProviders(<ApiDetailsTab onChange={onChange} values={VALUES} />);

    const sandbox = screen.getByRole('combobox', { name: 'Sandbox URL' });
    await user.click(sandbox);
    expect(screen.queryByRole('option')).not.toBeInTheDocument();

    await user.clear(sandbox);
    expect(onChange).toHaveBeenLastCalledWith({ ...VALUES, sandboxUrl: '' });
  });

  it('offers the gateway URL list only on a field that has URLs to pick from', () => {
    const options = [{ gatewayName: 'Gateway A', url: 'https://gw-a.example.com/loans' }];
    const { unmount } = renderWithProviders(<ApiDetailsTab values={VALUES} />);
    expect(screen.queryByRole('button', { name: 'Show gateway URLs' })).not.toBeInTheDocument();
    unmount();

    renderWithProviders(<ApiDetailsTab productionUrlOptions={options} values={VALUES} />);
    expect(screen.getAllByRole('button', { name: 'Show gateway URLs' })).toHaveLength(1);
  });

  it('reports the agent visibility switch as VISIBLE or HIDDEN', async () => {
    const onChange = vi.fn();
    const { user } = renderWithProviders(<ApiDetailsTab onChange={onChange} values={VALUES} />);

    const toggle = screen.getByRole('switch', { name: 'Make this API discoverable by AI agents' });
    expect(toggle).toBeChecked();

    await user.click(toggle);
    expect(onChange).toHaveBeenLastCalledWith({ ...VALUES, agentVisibility: 'HIDDEN' });
  });

  it('shows the switch off for a hidden API and turns it back on', async () => {
    const onChange = vi.fn();
    const hidden: DraftFormValues = { ...VALUES, agentVisibility: 'HIDDEN' };
    const { user } = renderWithProviders(<ApiDetailsTab onChange={onChange} values={hidden} />);

    const toggle = screen.getByRole('switch', { name: 'Make this API discoverable by AI agents' });
    expect(toggle).not.toBeChecked();

    await user.click(toggle);
    expect(onChange).toHaveBeenLastCalledWith({ ...hidden, agentVisibility: 'VISIBLE' });
  });

  it('shows the API name monogram as the thumbnail placeholder', () => {
    renderWithProviders(<ApiDetailsTab values={VALUES} />);

    expect(screen.getByText('LO')).toBeInTheDocument();
  });

  it('clears a URL with the clear button, and stops offering gateway URLs in custom mode', async () => {
    const onChange = vi.fn();
    const options = [{ gatewayName: 'Gateway A', url: 'https://gw-a.example.com/loans' }];
    const { user } = renderWithProviders(
      <ApiDetailsTab onChange={onChange} productionUrlOptions={options} values={VALUES} />,
    );

    // Hidden until the field is hovered or focused, which jsdom does not model.
    const clear = document.querySelector('[data-clear]');
    expect(clear).toHaveAttribute('aria-label', 'Clear');
    await user.click(clear as HTMLElement);
    expect(onChange).toHaveBeenLastCalledWith({ ...VALUES, productionUrl: '' });

    const [productionToggle, sandboxToggle] = screen.getAllByRole('button', { name: 'Enter a custom URL' });
    expect(sandboxToggle).toBeEnabled();
    await user.click(productionToggle);
    await user.click(screen.getByRole('combobox', { name: 'Production URL' }));
    expect(screen.queryByRole('option')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Choose from gateway URLs' })).toBeEnabled();
  });
});
