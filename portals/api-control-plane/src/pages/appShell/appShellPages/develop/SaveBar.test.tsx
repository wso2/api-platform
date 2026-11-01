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
import { SaveBar } from './SaveBar';

const renderBar = (props: Partial<React.ComponentProps<typeof SaveBar>> = {}) => {
  const onCancel = vi.fn();
  const onSave = vi.fn();
  const utils = renderWithProviders(
    <SaveBar dirty onCancel={onCancel} onSave={onSave} {...props} />,
  );
  return { ...utils, onCancel, onSave };
};

describe('SaveBar', () => {
  it('is not on the page while nothing is unsaved', () => {
    renderBar({ dirty: false });

    expect(screen.queryByRole('region')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Save changes' })).not.toBeInTheDocument();
  });

  it('says how many changes are unsaved, in the singular and the plural', () => {
    const { unmount } = renderBar({ changeCount: 1 });
    expect(screen.getByText('1 unsaved change')).toBeInTheDocument();
    unmount();

    renderBar({ changeCount: 3 });
    expect(screen.getByText('3 unsaved changes')).toBeInTheDocument();
  });

  it('only says there are changes when it is not given a count', () => {
    renderBar();

    expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
  });

  it('discards and saves through its two buttons', async () => {
    const { user, onCancel, onSave } = renderBar({ changeCount: 2 });

    await user.click(screen.getByRole('button', { name: 'Discard' }));
    await user.click(screen.getByRole('button', { name: 'Save changes' }));

    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledTimes(1);
  });

  it('holds the save button when the edits cannot be saved yet', () => {
    renderBar({ changeCount: 1, disabled: true });

    expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Discard' })).toBeEnabled();
  });

  it('stays up and locks both buttons while saving, even after the edits are marked saved', () => {
    renderBar({ dirty: false, saving: true });

    expect(screen.getByRole('button', { name: 'Saving…' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Discard' })).toBeDisabled();
  });

  it('uses a custom save label when given one', () => {
    renderBar({ changeCount: 1, label: 'Publish' });

    expect(screen.getByRole('button', { name: 'Publish' })).toBeInTheDocument();
  });
});
