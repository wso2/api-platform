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

import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen, waitFor } from '../../../../test/utils';
import { ConfirmDialog, type ConfirmDialogProps } from './ConfirmDialog';

const renderDialog = (props: Partial<ConfirmDialogProps> = {}) => {
  const onConfirm = vi.fn();
  const onCancel = vi.fn();
  const view = renderWithProviders(
    <ConfirmDialog
      message="This removes it."
      onCancel={onCancel}
      onConfirm={onConfirm}
      open
      title="Unpublish?"
      {...props}
    />
  );
  return { ...view, onCancel, onConfirm };
};

describe('ConfirmDialog', () => {
  it('shows its title and message, and nothing when closed', async () => {
    const { rerender } = renderDialog();
    expect(screen.getByRole('dialog', { name: 'Unpublish?' })).toHaveTextContent('This removes it.');

    rerender(<ConfirmDialog message="m" onCancel={vi.fn()} onConfirm={vi.fn()} open={false} title="Unpublish?" />);
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('confirms and cancels with the default labels', async () => {
    const { onCancel, onConfirm, user } = renderDialog();

    await user.click(screen.getByRole('button', { name: 'Confirm' }));
    expect(onConfirm).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('uses the labels it is given', () => {
    renderDialog({ cancelLabel: 'Keep it', confirmLabel: 'Unpublish' });

    expect(screen.getByRole('button', { name: 'Unpublish' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Keep it' })).toBeInTheDocument();
  });

  describe('with a phrase to type', () => {
    it('keeps Confirm off until the exact phrase is typed', async () => {
      const { onConfirm, user } = renderDialog({ confirmPhrase: 'Travel MCP' });
      const confirm = screen.getByRole('button', { name: 'Confirm' });
      const field = screen.getByLabelText('Type "Travel MCP" to confirm');

      expect(confirm).toBeDisabled();

      await user.type(field, 'travel mcp');
      expect(confirm).toBeDisabled();

      await user.clear(field);
      await user.type(field, 'Travel MCP');
      expect(confirm).toBeEnabled();

      await user.click(confirm);
      expect(onConfirm).toHaveBeenCalledTimes(1);
    });

    it('confirms on Enter once the phrase matches, and not before', async () => {
      const { onConfirm, user } = renderDialog({ confirmPhrase: 'Travel MCP' });
      const field = screen.getByLabelText('Type "Travel MCP" to confirm');

      await user.type(field, 'Travel{Enter}');
      expect(onConfirm).not.toHaveBeenCalled();

      await user.type(field, ' MCP{Enter}');
      expect(onConfirm).toHaveBeenCalledTimes(1);
    });

    it('uses the label it is given for the field', () => {
      renderDialog({ confirmInputLabel: 'Enter the name', confirmPhrase: 'Travel MCP' });

      expect(screen.getByLabelText('Enter the name')).toBeInTheDocument();
    });

    it('starts empty again when reopened', async () => {
      const { rerender, user } = renderDialog({ confirmPhrase: 'Travel MCP' });
      await user.type(screen.getByLabelText('Type "Travel MCP" to confirm'), 'Travel MCP');

      const props = { message: 'm', onCancel: vi.fn(), onConfirm: vi.fn(), title: 't', confirmPhrase: 'Travel MCP' };
      rerender(<ConfirmDialog {...props} open={false} />);
      rerender(<ConfirmDialog {...props} open />);

      expect(screen.getByLabelText('Type "Travel MCP" to confirm')).toHaveValue('');
      expect(screen.getByRole('button', { name: 'Confirm' })).toBeDisabled();
    });
  });

  it('locks both buttons while the action runs', () => {
    renderDialog({ loading: true });

    expect(screen.getByRole('button', { name: 'Confirm' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled();
  });
});
