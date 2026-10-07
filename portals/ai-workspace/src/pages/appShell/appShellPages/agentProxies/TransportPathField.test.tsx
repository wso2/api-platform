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
import { describe, it, expect, vi } from 'vitest';

import { renderWithProviders, screen } from '../../../../test/utils';
import TransportPathField, { validateTransportPath } from './TransportPathField';

const editable = (overrides: Partial<React.ComponentProps<typeof TransportPathField>> = {}) => {
  const props = {
    value: '/rpc',
    editing: true,
    onEditingChange: vi.fn(),
    onChange: vi.fn(),
    ...overrides,
  };
  return { props, ...renderWithProviders(<TransportPathField {...props} />) };
};

describe('validateTransportPath', () => {
  it.each([
    ['an empty value', '', 'Path is required.'],
    ['whitespace alone', '   ', 'Path is required.'],
    ['a path with no leading slash', 'rpc', 'Path must start with "/".'],
    ['a path containing a space', '/a b', 'Path cannot contain spaces.'],
  ])('rejects %s', (_label, value, message) => {
    expect(validateTransportPath(value)).toBe(message);
  });

  it.each(['/', '/rpc', '/v1/messages'])('accepts %s', (value) => {
    expect(validateTransportPath(value)).toBe('');
  });

  it('ignores surrounding whitespace when judging the path', () => {
    expect(validateTransportPath('  /rpc  ')).toBe('');
  });
});

describe('displaying a stored path', () => {
  it('shows the path as plain text with no input to edit it', () => {
    renderWithProviders(
      <TransportPathField
        value="/rpc"
        editing={false}
        onEditingChange={vi.fn()}
        onChange={vi.fn()}
      />
    );

    expect(screen.getByText('/rpc')).toBeInTheDocument();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });
});

describe('editing a path', () => {
  it('opens an input seeded with the stored path', () => {
    editable();

    expect(screen.getByRole('textbox')).toHaveValue('/rpc');
  });

  it('reports why an invalid path cannot be saved', async () => {
    const { user } = editable();
    const field = screen.getByRole('textbox');

    await user.clear(field);
    await user.type(field, 'rpc');

    expect(screen.getByText('Path must start with "/".')).toBeInTheDocument();
  });

  it('commits a valid path on Enter, trimmed, and closes the editor', async () => {
    const { props, user } = editable();
    const field = screen.getByRole('textbox');

    await user.clear(field);
    await user.type(field, '  /v1/messages  {Enter}');

    expect(props.onChange).toHaveBeenCalledWith('/v1/messages');
    expect(props.onEditingChange).toHaveBeenCalledWith(false);
  });

  it('keeps an invalid path out of the parent when Enter is pressed', async () => {
    const { props, user } = editable();
    const field = screen.getByRole('textbox');

    await user.clear(field);
    await user.type(field, 'rpc{Enter}');

    expect(props.onChange).not.toHaveBeenCalled();
  });

  it('abandons the edit on Escape without reporting a change', async () => {
    const { props, user } = editable();

    await user.type(screen.getByRole('textbox'), '{Escape}');

    expect(props.onEditingChange).toHaveBeenCalledWith(false);
    expect(props.onChange).not.toHaveBeenCalled();
  });

  it('commits a valid path when focus leaves the field', async () => {
    const { props, user } = editable();
    const field = screen.getByRole('textbox');

    await user.clear(field);
    await user.type(field, '/v1');
    await user.tab();

    expect(props.onChange).toHaveBeenCalledWith('/v1');
    expect(props.onEditingChange).toHaveBeenCalledWith(false);
  });

  it('closes without saving when focus leaves an invalid field', async () => {
    const { props, user } = editable();
    const field = screen.getByRole('textbox');

    await user.clear(field);
    await user.type(field, 'rpc');
    await user.tab();

    expect(props.onChange).not.toHaveBeenCalled();
    expect(props.onEditingChange).toHaveBeenCalledWith(false);
  });

  it('reseeds the draft from the stored path each time editing reopens', () => {
    const props = {
      value: '/rpc',
      editing: false,
      onEditingChange: vi.fn(),
      onChange: vi.fn(),
    };
    const { rerender } = renderWithProviders(<TransportPathField {...props} />);

    rerender(<TransportPathField {...props} value="/changed" editing />);

    expect(screen.getByRole('textbox')).toHaveValue('/changed');
  });
});
