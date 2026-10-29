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

import { fireEvent } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '../../../../test/utils';
import { McpProxyDetailsTab, type McpProxyDetailsTabProps } from './McpProxyDetailsTab';
import { emptyDraftFormValues, type DraftFormValues } from './mcpPublishForm';

const values: DraftFormValues = {
  ...emptyDraftFormValues,
  displayName: 'Travel MCP',
  version: '1.0.0',
  description: 'Plans trips.',
  productionUrl: 'https://gw.example.com/travel/mcp',
  sandboxUrl: 'https://sandbox.example.com/mcp',
};

const options = [
  { gatewayName: 'Prod', url: 'https://gw.example.com/travel/mcp' },
  { gatewayName: 'Stage', url: 'https://stage.example.com/travel/mcp' },
];

const agentSwitch = () => screen.getByLabelText(/discoverable by AI agents/);

const renderTab = (props: Partial<McpProxyDetailsTabProps> = {}) => {
  const onChange = vi.fn();
  const onBlurField = vi.fn();
  const view = renderWithProviders(
    <McpProxyDetailsTab onBlurField={onBlurField} onChange={onChange} values={values} {...props} />
  );
  return { ...view, onBlurField, onChange };
};

describe('McpProxyDetailsTab', () => {
  it('shows every value, with the name’s monogram standing in until there is a thumbnail', () => {
    renderTab();

    expect(screen.getByLabelText(/^Name/)).toHaveValue('Travel MCP');
    expect(screen.getByLabelText(/^Version/)).toHaveValue('1.0.0');
    expect(screen.getByLabelText('Description')).toHaveValue('Plans trips.');
    expect(screen.getByLabelText('Production URL')).toHaveValue('https://gw.example.com/travel/mcp');
    expect(screen.getByLabelText('Sandbox URL')).toHaveValue('https://sandbox.example.com/mcp');
    expect(screen.getByText('TR')).toBeInTheDocument();
  });

  it('marks the name and version as required', () => {
    renderTab();

    expect(screen.getByLabelText(/^Name/)).toBeRequired();
    expect(screen.getByLabelText(/^Version/)).toBeRequired();
  });

  describe('editing', () => {
    it.each([
      ['Name', /^Name/, 'displayName', 'Trips'],
      ['Version', /^Version/, 'version', '2.0.0'],
      ['Description', 'Description', 'description', 'New text'],
      ['Sandbox URL', 'Sandbox URL', 'sandboxUrl', 'https://s.example.com'],
    ])('reports a change to %s with the rest of the form untouched', (_name, label, field, next) => {
      const { onChange } = renderTab();

      fireEvent.change(screen.getByLabelText(label), { target: { value: next } });

      expect(onChange).toHaveBeenCalledWith({ ...values, [field]: next });
    });

    it('reports a blur on a field, so its message can wait until the user leaves it', () => {
      const { onBlurField } = renderTab();

      fireEvent.blur(screen.getByLabelText(/^Name/));

      expect(onBlurField).toHaveBeenCalledWith('displayName');
    });

    it('shows each validation message beside its field', () => {
      renderTab({ errors: { displayName: 'Enter a name.', productionUrl: 'Enter a full URL.' } });

      expect(screen.getByText('Enter a name.')).toBeInTheDocument();
      expect(screen.getByText('Enter a full URL.')).toBeInTheDocument();
      expect(screen.getByLabelText(/^Name/)).toBeInvalid();
    });

    it('turns agent discoverability off and on', () => {
      const { onChange, rerender } = renderTab();
      const toggle = agentSwitch();
      expect(toggle).toBeChecked();

      fireEvent.click(toggle);
      expect(onChange).toHaveBeenLastCalledWith({ ...values, agentVisibility: 'HIDDEN' });

      rerender(<McpProxyDetailsTab onChange={onChange} values={{ ...values, agentVisibility: 'HIDDEN' }} />);
      expect(agentSwitch()).not.toBeChecked();
      fireEvent.click(agentSwitch());
      expect(onChange).toHaveBeenLastCalledWith({ ...values, agentVisibility: 'VISIBLE' });
    });

    it('turns every field off while an action runs', () => {
      renderTab({ disabled: true });

      expect(screen.getByLabelText(/^Name/)).toBeDisabled();
      expect(screen.getByLabelText('Description')).toBeDisabled();
      expect(screen.getByLabelText('Production URL')).toBeDisabled();
      expect(agentSwitch()).toBeDisabled();
    });
  });

  describe('the URL fields', () => {
    it('can be cleared with the clear button', () => {
      const { onChange } = renderTab();

      // Hidden until the field is hovered or focused, so it has no accessible name to find it by here.
      fireEvent.click(screen.getAllByLabelText('Clear')[0]);

      expect(onChange).toHaveBeenCalledWith({ ...values, productionUrl: '' });
    });

    it('offers no clear button for an empty field', () => {
      renderTab({ values: { ...values, productionUrl: '', sandboxUrl: '' } });

      expect(screen.queryByLabelText('Clear')).not.toBeInTheDocument();
    });

    it('offers the gateway URLs for the Production URL, each named by its gateway', () => {
      renderTab({ productionUrlOptions: options });

      fireEvent.click(screen.getByRole('button', { name: 'Show gateway URLs' }));

      expect(screen.getByRole('option', { name: /stage\.example\.com.*Stage/ })).toBeInTheDocument();
    });

    it('picks a gateway URL', () => {
      const { onChange } = renderTab({ productionUrlOptions: options });

      fireEvent.click(screen.getByRole('button', { name: 'Show gateway URLs' }));
      fireEvent.click(screen.getByRole('option', { name: /stage\.example\.com/ }));

      expect(onChange).toHaveBeenCalledWith({ ...values, productionUrl: 'https://stage.example.com/travel/mcp' });
    });

    it('has nothing to pick from for the Sandbox URL', () => {
      renderTab({ productionUrlOptions: options });

      expect(screen.getAllByRole('button', { name: 'Show gateway URLs' })).toHaveLength(1);
    });

    it('offers nothing to pick when the proxy is not deployed anywhere, but still lets a URL be typed', () => {
      const { onChange } = renderTab({ productionUrlOptions: [] });

      expect(screen.queryByRole('button', { name: 'Show gateway URLs' })).not.toBeInTheDocument();

      fireEvent.change(screen.getByLabelText('Production URL'), { target: { value: 'https://own.example.com' } });
      expect(onChange).toHaveBeenCalledWith({ ...values, productionUrl: 'https://own.example.com' });
    });

    it('can switch to typing a custom URL and back to picking', () => {
      renderTab({ productionUrlOptions: options });

      fireEvent.click(screen.getAllByRole('button', { name: 'Enter a custom URL' })[0]);
      expect(screen.queryByRole('button', { name: 'Show gateway URLs' })).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', { name: 'Choose from gateway URLs' }));
      expect(screen.getByRole('button', { name: 'Show gateway URLs' })).toBeInTheDocument();
    });
  });

  describe('read-only, for the published version', () => {
    it('shows the values without letting any be changed', () => {
      const { onChange } = renderTab({ readOnly: true });

      expect(screen.getByLabelText(/^Name/)).toHaveAttribute('readonly');
      expect(screen.getByLabelText('Description')).toHaveAttribute('readonly');
      expect(agentSwitch()).toBeDisabled();
      expect(onChange).not.toHaveBeenCalled();
    });

    it('has no clear, pick or custom-URL buttons', () => {
      renderTab({ productionUrlOptions: options, readOnly: true });

      expect(screen.queryByLabelText('Clear')).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Show gateway URLs' })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Enter a custom URL' })).not.toBeInTheDocument();
    });

    it('does not suggest what to write in an empty description', () => {
      renderTab({ readOnly: true, values: { ...values, description: '' } });

      expect(screen.getByLabelText('Description')).not.toHaveAttribute('placeholder');
    });
  });

  it('shows the thumbnail image in place of the monogram when there is one', () => {
    renderTab({ thumbnailUrl: 'blob:test/1' });

    expect(screen.getByRole('img', { name: 'Travel MCP' })).toHaveAttribute('src', 'blob:test/1');
    expect(screen.queryByText('TR')).not.toBeInTheDocument();
  });

  it('reports the image picked to replace the thumbnail, and lets the same one be picked again', async () => {
    const onThumbnailPick = vi.fn();
    const { container, user } = renderTab({ onThumbnailPick });
    const picker = container.querySelector('input[type="file"]') as HTMLInputElement;
    const image = new File(['png'], 'logo.png', { type: 'image/png' });

    expect(screen.getByRole('button', { name: 'Change thumbnail' })).toBeInTheDocument();
    expect(picker).toHaveAttribute('accept', 'image/png,image/jpeg');

    await user.upload(picker, image);
    expect(onThumbnailPick).toHaveBeenLastCalledWith(image);

    await user.upload(picker, image);
    expect(onThumbnailPick).toHaveBeenCalledTimes(2);
  });

  it('opens the file picker from the button', async () => {
    const { container, user } = renderTab({ onThumbnailPick: vi.fn() });
    const picker = container.querySelector('input[type="file"]') as HTMLInputElement;
    const click = vi.spyOn(picker, 'click');

    await user.click(screen.getByRole('button', { name: 'Change thumbnail' }));

    expect(click).toHaveBeenCalledTimes(1);
  });

  it('does not offer to change the thumbnail when it is read-only, disabled or has no handler', () => {
    const onThumbnailPick = vi.fn();
    const { rerender } = renderWithProviders(<McpProxyDetailsTab onThumbnailPick={onThumbnailPick} readOnly values={values} />);
    expect(screen.queryByRole('button', { name: 'Change thumbnail' })).not.toBeInTheDocument();

    rerender(<McpProxyDetailsTab disabled onThumbnailPick={onThumbnailPick} values={values} />);
    expect(screen.queryByRole('button', { name: 'Change thumbnail' })).not.toBeInTheDocument();

    rerender(<McpProxyDetailsTab values={values} />);
    expect(screen.queryByRole('button', { name: 'Change thumbnail' })).not.toBeInTheDocument();
  });

  it('shows an icon instead of initials when the name has no letters yet', () => {
    renderTab({ values: { ...values, displayName: '' } });

    expect(screen.queryByText('TR')).not.toBeInTheDocument();
  });
});
